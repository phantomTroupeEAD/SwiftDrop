import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/shared/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[8px] text-[13.5px] font-medium transition-colors duration-150 active:scale-[0.98] transition-transform duration-100 ease-out focus-visible:outline-none focus-visible:shadow-[0_0_0_2px_var(--surface),0_0_0_4px_var(--ring)] disabled:pointer-events-none disabled:opacity-45 [&_svg]:pointer-events-none [&_svg]:size-[15px] [&_svg]:shrink-0",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-none border border-transparent hover:bg-[var(--primary-hover)]",
        destructive:
          "bg-[var(--status-error-bg)] text-[var(--status-error-ink)] border border-[var(--status-error-border)] shadow-none hover:bg-[var(--status-error-border)]",
        outline:
          "border border-border bg-transparent shadow-none hover:bg-surface-subtle text-foreground",
        secondary:
          "bg-surface-subtle text-foreground border border-transparent shadow-none hover:bg-surface-active",
        ghost:
          "bg-transparent shadow-none hover:bg-surface-subtle text-foreground",
        link:
          "text-primary underline-offset-4 hover:underline p-0 h-auto shadow-none",
      },
      size: {
        default: "min-h-[38px] touch:min-h-[44px] px-4 py-2",
        sm: "min-h-[30px] rounded-[8px] px-2.5 py-1 text-xs",
        lg: "min-h-[44px] rounded-[8px] px-6 text-sm",
        icon: "h-[38px] w-[38px] touch:h-[44px] touch:w-[44px] p-0",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  }
);
Button.displayName = "Button";

export { Button, buttonVariants };
