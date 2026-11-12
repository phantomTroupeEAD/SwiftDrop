import { cn } from "@/shared/lib/utils"

function Skeleton({
  className,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-[8px] bg-surface-subtle after:absolute after:inset-0 after:-translate-x-full after:animate-shimmer after:bg-gradient-to-r after:from-transparent after:via-surface/40 after:to-transparent",
        className
      )}
      {...props}
    />
  )
}

export { Skeleton }
