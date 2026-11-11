import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = {title: "ResiliencePlane · Operations", description: "Local reliability control-plane console"};
export default function RootLayout({children}: Readonly<{children: React.ReactNode}>) {
  return <html lang="en"><body>{children}</body></html>;
}
