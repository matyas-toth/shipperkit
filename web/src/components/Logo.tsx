import { splitProps, type JSX } from "solid-js";

export default function Logo(props: JSX.ImgHTMLAttributes<HTMLImageElement>) {
  const [local, rest] = splitProps(props, ["class"]);
  return <img src="/shipper.svg" width="20" height="20" alt="" aria-hidden="true" class={"icon " + (local.class ?? "")} {...rest} />;
}
