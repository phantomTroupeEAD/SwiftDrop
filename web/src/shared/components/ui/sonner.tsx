import * as React from "react";
import { Toaster as Sonner } from "sonner";

type ToasterProps = React.ComponentProps<typeof Sonner>;

const Toaster = ({ ...props }: ToasterProps) => {
  const [theme, setTheme] = React.useState<"light" | "dark">(() => {
    if (typeof document !== "undefined") {
      return document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
    }
    return "light";
  });

  React.useEffect(() => {
    const observer = new MutationObserver(() => {
      const currentTheme = document.documentElement.getAttribute("data-theme") === "dark" ? "dark" : "light";
      setTheme(currentTheme);
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => observer.disconnect();
  }, []);

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      toastOptions={{
        classNames: {
          toast:
            "group toast group-[.toaster]:bg-surface group-[.toaster]:text-text group-[.toaster]:border group-[.toaster]:border-border group-[.toaster]:rounded-[8px] group-[.toaster]:shadow-none",
          description: "group-[.toast]:text-text-muted",
          actionButton:
            "group-[.toast]:bg-primary group-[.toast]:text-primary-foreground group-[.toast]:rounded-[6px] group-[.toast]:text-xs",
          cancelButton:
            "group-[.toast]:bg-surface-subtle group-[.toast]:text-text-muted group-[.toast]:rounded-[6px] group-[.toast]:text-xs",
        },
      }}
      {...props}
    />
  );
};

export { Toaster };
