import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";
export async function GET(request: NextRequest) {
  const authorization = request.headers.get("authorization") ?? "";
  if (!/^Bearer [^\s]{16,4096}$/.test(authorization)) {
    return NextResponse.json({error: "A local development access token is required."}, {status: 401});
  }
  try {
    // Backend origin is server configuration, never a browser-controlled query parameter.
    const origin = new URL(process.env.TRS_API_URL ?? "http://127.0.0.1:8080");
    if (!["http:", "https:"].includes(origin.protocol) || origin.username || origin.password) throw new Error("Invalid backend URL");
    const upstream = await fetch(new URL("/v1/snapshot", origin), {
      headers: {authorization, accept: "application/json"}, cache: "no-store",
      redirect: "error", signal: AbortSignal.timeout(8000)
    });
    if (!upstream.ok) {
      const status = upstream.status === 401 || upstream.status === 403 ? upstream.status : 502;
      return NextResponse.json({error: status === 502 ? "The management API could not return a snapshot." : "The management API rejected this access token."}, {status});
    }
    if (!upstream.headers.get("content-type")?.includes("application/json")) throw new Error("Invalid content type");
    const reader = upstream.body?.getReader();
    if (!reader) throw new Error("Empty snapshot response");
    const chunks: Uint8Array[] = [];
    let total = 0;
    while (true) {
      const {done, value} = await reader.read();
      if (done) break;
      total += value.byteLength;
      if (total > 4_000_000) {
        await reader.cancel();
        throw new Error("Snapshot limit exceeded");
      }
      chunks.push(value);
    }
    const bytes = new Uint8Array(total);
    let offset = 0;
    for (const chunk of chunks) {bytes.set(chunk, offset); offset += chunk.byteLength;}
    const text = new TextDecoder().decode(bytes);
    return new NextResponse(text, {status: 200, headers: {"Content-Type": "application/json", "Cache-Control": "no-store"}});
  } catch {
    return NextResponse.json({error: "The management API is unavailable or its response exceeded the console limit."}, {status: 502});
  }
}
