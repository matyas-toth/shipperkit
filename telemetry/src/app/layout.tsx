import type { Metadata, Viewport } from "next";
import "@fontsource-variable/inter";
import "./globals.css";

export const metadata: Metadata = {
  title: { default: "ShipperKit Telemetry", template: "%s · ShipperKit Telemetry" },
  icons: { icon: "/shipper.svg" },
  robots: { index: false, follow: false },
};

export const viewport: Viewport = { themeColor: "#070808", colorScheme: "dark" };

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-dvh">{children}</body>
    </html>
  );
}
