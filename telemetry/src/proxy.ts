import { NextResponse, type NextRequest } from "next/server";
import { SESSION_COOKIE, verifySession } from "@/lib/session";

// An optimistic gate so signed-out visitors never see the dashboard shell.
// Data reads verify the session again in requireAdmin.
export async function proxy(request: NextRequest) {
  const signedIn = await verifySession(request.cookies.get(SESSION_COOKIE)?.value);
  const onLogin = request.nextUrl.pathname === "/login";
  if (!signedIn && !onLogin) return NextResponse.redirect(new URL("/login", request.url));
  if (signedIn && onLogin) return NextResponse.redirect(new URL("/", request.url));
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!api/v1/ingest|api/health|_next/|shipper.svg|icon.svg|favicon.ico).*)"],
};
