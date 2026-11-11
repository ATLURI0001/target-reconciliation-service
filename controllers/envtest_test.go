package controllers

import (
	"context"
	"errors"
	"fmt"
	"github.com/ATLURI0001/resilienceplane/internal/core"
	"github.com/ATLURI0001/resilienceplane/internal/store"
	"github.com/jackc/pgx/v5"
	"io"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/rest"
	"net/url"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

// This exercises a real API server, not a scheduler, kubelet or a running Pod.
// A Kind end-to-end run is separately required to verify an actual rollout.
func TestEnvtestAtomicPatch(t *testing.T) {
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS absent; real API-server integration not run")
	}
	env := &envtest.Environment{CRDDirectoryPaths: []string{"../config/crd"}, ErrorIfCRDPathMissing: true}
	// Explicit loopback advertise address also supports isolated development
	// environments without a default route; no external interface is exposed.
	env.ControlPlane.GetAPIServer().Configure().Set("advertise-address", "127.0.0.1")
	cfg, err := env.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := env.Stop(); err != nil {
			t.Error(err)
		}
	})
	s := scheme(t)
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	c, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: DemoNamespace}}); err != nil {
		t.Fatal(err)
	}
	d := sampleDeployment()
	d.UID = ""
	d.ResourceVersion = ""
	d.Generation = 0
	if err := c.Create(ctx, d); err != nil {
		t.Fatal(err)
	}
	r := &TargetReconciler{Client: c, ClusterID: "local"}
	p := plan(r, d)
	stale := d.DeepCopy()
	d.Annotations = map[string]string{"concurrent": "write"}
	if err := c.Update(ctx, d); err != nil {
		t.Fatal(err)
	}
	if err := r.Execute(ctx, stale, p); err == nil {
		t.Fatal("stale atomic patch must fail")
	}
	// A new read may refresh resourceVersion while retaining the approved spec
	// generation. The API server must still serialize the final patch atomically.
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
		t.Fatal(err)
	}
	p = plan(r, d)
	if err := r.Execute(ctx, d, p); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
		t.Fatal(err)
	}
	if d.Spec.Template.Spec.Containers[0].Image != goodImage {
		t.Fatal("image not patched")
	}
	t.Run("NamespaceRBAC", func(t *testing.T) {
		f, err := os.Open("../config/rbac/controller.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		decoder := utilyaml.NewYAMLOrJSONDecoder(f, 4096)
		for {
			object := &unstructured.Unstructured{}
			err := decoder.Decode(object)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(object.Object) == 0 {
				continue
			}
			if err := c.Create(ctx, object); err != nil {
				t.Fatal(err)
			}
		}
		scopedConfig := rest.CopyConfig(cfg)
		scopedConfig.Impersonate = rest.ImpersonationConfig{UserName: "system:serviceaccount:rp-demo:resilienceplane-controller", Groups: []string{"system:serviceaccounts", "system:serviceaccounts:rp-demo", "system:authenticated"}}
		scoped, err := client.New(scopedConfig, client.Options{Scheme: s})
		if err != nil {
			t.Fatal(err)
		}
		var allowed appsv1.Deployment
		if err := scoped.Get(ctx, client.ObjectKeyFromObject(d), &allowed); err != nil {
			t.Fatalf("scoped deployment read failed: %v", err)
		}
		allowed.Annotations = map[string]string{"rbac-check": "allowed"}
		if err := scoped.Patch(ctx, &allowed, client.MergeFrom(d.DeepCopy())); err != nil {
			t.Fatalf("scoped deployment patch failed: %v", err)
		}
		var secret corev1.Secret
		if err := scoped.Get(ctx, types.NamespacedName{Namespace: DemoNamespace, Name: "not-readable"}, &secret); !apierrors.IsForbidden(err) {
			t.Fatalf("expected secret denial, got %v", err)
		}
		var protected appsv1.Deployment
		if err := scoped.Get(ctx, types.NamespacedName{Namespace: "kube-system", Name: "protected"}, &protected); !apierrors.IsForbidden(err) {
			t.Fatalf("expected protected-namespace denial, got %v", err)
		}
	})
	t.Run("CRDSchema", func(t *testing.T) {
		invalid := Target(DemoNamespace, "invalid", "catalog", "catalog", "nginx:latest")
		if err := c.Create(ctx, invalid); err == nil || !apierrors.IsInvalid(err) {
			t.Fatalf("CRD admitted mutable tag or returned unrelated error: %v", err)
		}
	})
	t.Run("WorkflowWithExplicitStatusInput", func(t *testing.T) {
		repository := store.NewMemory()
		testRealAPIWorkflow(t, c, repository, "memory-catalog", func() core.Store { return repository })
	})
	t.Run("WorkflowWithPostgresAndExplicitStatusInput", func(t *testing.T) {
		dsn := os.Getenv("TEST_DATABASE_URL")
		if dsn == "" {
			t.Skip("TEST_DATABASE_URL absent; real PostgreSQL workflow not run")
		}
		ctx := context.Background()
		admin, err := pgx.Connect(ctx, dsn)
		if err != nil {
			t.Fatal("connect envtest PostgreSQL:", err)
		}
		schema := fmt.Sprintf("rp_envtest_%d", time.Now().UnixNano())
		quoted := pgx.Identifier{schema}.Sanitize()
		if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
			admin.Close(ctx)
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
			if err != nil {
				t.Error(err)
			}
			admin.Close(context.Background())
		})
		isolatedDSN := dsn
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			u, err := url.Parse(dsn)
			if err != nil {
				t.Fatal(err)
			}
			q := u.Query()
			q.Set("search_path", schema)
			u.RawQuery = q.Encode()
			isolatedDSN = u.String()
		} else {
			isolatedDSN = dsn + " search_path=" + schema
		}
		repository, err := store.NewPostgres(ctx, isolatedDSN)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(repository.Close)
		restart := func() core.Store {
			repository.Close()
			replacement, err := store.NewPostgres(ctx, isolatedDSN)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(replacement.Close)
			return replacement
		}
		testRealAPIWorkflow(t, c, repository, "postgres-catalog", restart)
	})
}

// A real API server validates and serializes these resources. There are no
// kubelets or Deployment controller in envtest: the test supplies status and
// therefore does not establish live workload recovery or a measured RTO.
func testRealAPIWorkflow(t *testing.T, c client.Client, repository core.Store, name string, restart func() core.Store) {
	ctx := context.Background()
	d := sampleDeployment()
	d.Name = name
	d.UID = ""
	d.ResourceVersion = ""
	d.Generation = 0
	if err := c.Create(ctx, d); err != nil {
		t.Fatal(err)
	}
	target := Target(DemoNamespace, name+"-target", d.Name, "catalog", goodImage)
	if err := c.Create(ctx, target); err != nil {
		t.Fatal(err)
	}
	svc := core.NewService(repository)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	svc.Now = func() time.Time { return now }
	who := core.Identity{Subject: "envtest-operator", Role: "operator"}
	if err := svc.Register(ctx, who, core.Cluster{ID: "envtest", Environment: "test"}); err != nil {
		t.Fatal(err)
	}
	policy := core.DefaultPolicy()
	policy.DetectionSeconds = 1
	policy.DryRun = false
	if err := svc.SetPolicy(ctx, who, policy); err != nil {
		t.Fatal(err)
	}
	r := &TargetReconciler{Client: c, Workflow: svc, ClusterID: "envtest", Now: func() time.Time { return now }}
	request := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: DemoNamespace, Name: target.GetName()}}
	reconcile := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, request); err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	now = now.Add(2 * time.Second)
	reconcile()
	state, err := repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Incidents) != 1 {
		t.Fatalf("incidents=%v", state.Incidents)
	}
	var incident core.Incident
	for _, value := range state.Incidents {
		incident = value
	}
	if incident.State != "AwaitingApproval" {
		t.Fatalf("incident=%+v", incident)
	}
	if err := svc.Approve(ctx, core.Identity{Subject: "reader", Role: "viewer"}, incident.ID); !errors.Is(err, core.ErrDenied) {
		t.Fatalf("viewer authorization=%v", err)
	}
	reconcile()
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
		t.Fatal(err)
	}
	if d.Spec.Template.Spec.Containers[0].Image == goodImage {
		t.Fatal("viewer caused mutation")
	}
	if err := svc.Approve(ctx, who, incident.ID); err != nil {
		t.Fatal(err)
	}
	reconcile()
	state, err = repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "Verifying" {
		t.Fatalf("incident after rollback=%+v", state.Incidents[incident.ID])
	}
	if err := c.Get(ctx, client.ObjectKeyFromObject(d), d); err != nil {
		t.Fatal(err)
	}
	if d.Spec.Template.Spec.Containers[0].Image != goodImage {
		t.Fatal("approved image not persisted by API server")
	}
	// Supply status, explicitly, because envtest never starts Pods.
	d.Status.Replicas = 1
	d.Status.ReadyReplicas = 1
	d.Status.AvailableReplicas = 1
	d.Status.UpdatedReplicas = 1
	d.Status.ObservedGeneration = d.Generation
	if err := c.Status().Update(ctx, d); err != nil {
		t.Fatal(err)
	}
	repository = restart()
	restarted := core.NewService(repository)
	restarted.Now = func() time.Time { return now }
	r.Workflow = restarted
	now = now.Add(time.Second)
	reconcile()
	state, err = repository.View(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if state.Incidents[incident.ID].State != "Resolved" {
		t.Fatalf("verification=%+v", state.Incidents[incident.ID])
	}
	if len(state.Audit) < 6 {
		t.Fatalf("audit missing workflow: %+v", state.Audit)
	}
}
