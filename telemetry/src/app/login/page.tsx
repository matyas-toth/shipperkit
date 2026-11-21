import type { Metadata } from "next";
import { Frame, Topbar } from "@/components/Shell";
import { LoginForm } from "./LoginForm";

export const metadata: Metadata = { title: "Sign in" };

export default function LoginPage() {
  return (
    <>
      <Topbar />
      <Frame>
        <div className="grid min-h-[calc(100dvh-180px)] place-items-center">
          <section className="w-full max-w-[400px] rounded-[16px] border border-border bg-surface p-[30px] shadow-[0_24px_60px_#00000066,inset_0_0.5px_0_#ffffff14]">
            <h1 className="mb-6 text-[27px] leading-8 font-[540] tracking-[-0.7px]">Sign in</h1>
            <LoginForm />
          </section>
        </div>
      </Frame>
    </>
  );
}
