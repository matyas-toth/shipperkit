import { IconLogout } from "@tabler/icons-react";
import { Frame, Topbar } from "@/components/Shell";
import { Nav } from "@/components/Nav";
import { logout } from "../login/actions";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <Topbar>
        <Nav />
        <form action={logout} className="ml-auto">
          <button type="submit" aria-label="Sign out" title="Sign out" className="grid size-8 place-items-center rounded-control text-muted transition-colors hover:bg-surface hover:text-text">
            <IconLogout size={17} stroke={1.75} />
          </button>
        </form>
      </Topbar>
      <Frame>{children}</Frame>
    </>
  );
}
