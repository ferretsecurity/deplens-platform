import type { Metadata } from "next";
import { IBM_Plex_Mono, Manrope } from "next/font/google";

import { QueryProvider } from "@/providers/query-provider";
import "./globals.css";

const sans = Manrope({
  subsets: ["latin"],
  variable: "--font-sans"
});

const mono = IBM_Plex_Mono({
  subsets: ["latin"],
  variable: "--font-mono",
  weight: ["400", "500"]
});

export const metadata: Metadata = {
  title: "deplens",
  description: "Explore scan results and dependencies"
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className={`${sans.className} ${mono.variable} bg-background text-foreground antialiased`}>
        <QueryProvider>{children}</QueryProvider>
      </body>
    </html>
  );
}
