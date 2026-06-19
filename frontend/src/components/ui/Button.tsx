import * as React from "react";
import { Slot } from "@radix-ui/react-slot";

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "danger" | "ghost";
  size?: "default" | "sm" | "icon";
  asChild?: boolean;
};

export function Button({
  className = "",
  variant = "primary",
  size = "default",
  asChild = false,
  ...props
}: ButtonProps) {
  const Comp = asChild ? Slot : "button";
  const classes = [
    "btn",
    `btn-${variant}`,
    size === "sm" ? "btn-sm" : "",
    size === "icon" ? "btn-icon" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  return <Comp className={classes} {...props} />;
}
