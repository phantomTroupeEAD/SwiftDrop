import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/shared/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center gap-1.5 rounded-[8px] border px-2 py-0.5 text-xs font-medium leading-[1.35] whitespace-nowrap transition-colors focus:outline-none focus:ring-0 shadow-none",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary text-primary-foreground shadow-none hover:bg-[var(--primary-hover)]",
        secondary:
          "border-transparent bg-surface-subtle text-foreground shadow-none hover:bg-surface-active",
        destructive:
          "border-status-error-border bg-status-error-bg text-status-error-ink shadow-none hover:bg-[var(--status-error-bg)]",
        outline: "border-border text-foreground shadow-none",
        neutral:
          "border-border bg-surface-subtle text-text-muted shadow-none",
        active:
          "border-transparent bg-[var(--primary-tint)] text-primary shadow-none",
        success:
          "border-status-success-border bg-status-success-bg text-status-success-ink shadow-none",
        warning:
          "border-status-warning-border bg-status-warning-bg text-status-warning-ink shadow-none",
        error:
          "border-status-error-border bg-status-error-bg text-status-error-ink shadow-none",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {
  showDot?: boolean
}

function Badge({ className, variant, showDot, children, ...props }: BadgeProps) {
  const hasDot = showDot ?? (variant === "neutral" || variant === "active" || variant === "success" || variant === "warning" || variant === "error")

  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props}>
      {hasDot && <span className="w-1.5 h-1.5 rounded-full bg-current shrink-0" />}
      {children}
    </div>
  )
}

export { Badge, badgeVariants }
