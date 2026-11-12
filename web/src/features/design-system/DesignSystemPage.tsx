import React, { useState, useEffect } from 'react';
import {
  Button,
} from '@/shared/components/ui/button';
import {
  Badge,
} from '@/shared/components/ui/badge';
import {
  Input,
} from '@/shared/components/ui/input';
import {
  Label,
} from '@/shared/components/ui/label';
import {
  Textarea,
} from '@/shared/components/ui/textarea';
import {
  Switch,
} from '@/shared/components/ui/switch';
import {
  Checkbox,
} from '@/shared/components/ui/checkbox';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
  SelectGroup,
  SelectLabel,
} from '@/shared/components/ui/select';
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from '@/shared/components/ui/card';
import {
  Table,
  TableHeader,
  TableBody,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
} from '@/shared/components/ui/table';
import {
  Tabs,
  TabsList,
  TabsTrigger,
  TabsContent,
} from '@/shared/components/ui/tabs';
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
  DialogClose,
} from '@/shared/components/ui/dialog';
import {
  Popover,
  PopoverTrigger,
  PopoverContent,
} from '@/shared/components/ui/popover';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/shared/components/ui/dropdown-menu';
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
  TooltipProvider,
} from '@/shared/components/ui/tooltip';
import {
  Accordion,
  AccordionItem,
  AccordionTrigger,
  AccordionContent,
} from '@/shared/components/ui/accordion';
import {
  Skeleton,
} from '@/shared/components/ui/skeleton';
import {
  Avatar,
  AvatarImage,
  AvatarFallback,
} from '@/shared/components/ui/avatar';
import {
  ScrollArea,
} from '@/shared/components/ui/scroll-area';
import {
  Separator,
} from '@/shared/components/ui/separator';
import { toast, Toaster } from 'sonner';
import {
  Sun,
  Moon,
  Sparkles,
  AlertTriangle,
  Package,
  Send,
  Loader2,
} from 'lucide-react';

type RoleScope = 'PUBLIC' | 'CUSTOMER' | 'MERCHANT' | 'DISPATCHER' | 'COURIER' | 'ADMIN';

export const DesignSystemPage: React.FC = () => {
  const [currentRole, setCurrentRole] = useState<RoleScope>('PUBLIC');
  const [isDark, setIsDark] = useState<boolean>(() => {
    if (typeof document !== 'undefined') {
      return document.documentElement.getAttribute('data-theme') === 'dark';
    }
    return false;
  });
  const [switchChecked, setSwitchChecked] = useState<boolean>(true);
  const [checkboxChecked, setCheckboxChecked] = useState<boolean>(true);
  const [dialogOpen, setDialogOpen] = useState<boolean>(false);

  useEffect(() => {
    const root = document.documentElement;
    if (isDark) {
      root.setAttribute('data-theme', 'dark');
    } else {
      root.removeAttribute('data-theme');
    }
  }, [isDark]);

  const toggleTheme = () => {
    setIsDark((prev) => !prev);
  };

  return (
    <TooltipProvider>
      <Toaster position="bottom-right" />
      <div data-role={currentRole} className="min-h-screen bg-canvas text-text transition-colors duration-200">
        {/* Top Control Bar */}
        <header className="sticky top-0 z-40 border-b border-border bg-surface px-6 py-3.5 backdrop-blur-md">
          <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-4">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-[8px] bg-primary text-primary-foreground font-semibold text-sm">
                SD
              </div>
              <div>
                <h1 className="text-base font-semibold leading-tight text-text">SwiftDrop Design System Studio</h1>
                <p className="text-xs text-text-muted">Parity Studio & Component Verification (Apple HIG + shadcn)</p>
              </div>
            </div>

            {/* Studio Controls: Theme & Role Switcher */}
            <div className="flex flex-wrap items-center gap-3">
              {/* Role Switcher */}
              <div className="flex items-center gap-1.5 rounded-[8px] border border-border bg-surface-subtle p-1">
                {(['PUBLIC', 'CUSTOMER', 'MERCHANT', 'DISPATCHER', 'COURIER', 'ADMIN'] as RoleScope[]).map((role) => (
                  <Button
                    key={role}
                    variant={currentRole === role ? 'default' : 'ghost'}
                    size="sm"
                    className="h-7 px-2.5 text-xs"
                    onClick={() => setCurrentRole(role)}
                  >
                    {role}
                  </Button>
                ))}
              </div>

              {/* Theme Toggle */}
              <Button
                variant="outline"
                size="sm"
                className="h-9 gap-2 px-3"
                onClick={toggleTheme}
              >
                {isDark ? <Sun className="h-4 w-4 text-primary" /> : <Moon className="h-4 w-4 text-primary" />}
                <span className="text-xs font-medium">{isDark ? 'Dark Mode' : 'Light Mode'}</span>
              </Button>
            </div>
          </div>
        </header>

        {/* Main Content Showcase */}
        <main className="mx-auto max-w-7xl space-y-12 px-6 py-8">
          {/* Banner: Active Role Context */}
          <Card className="border-border bg-surface">
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-primary">
                  <Sparkles className="h-4 w-4" />
                  <span>Interactive Context Scoping</span>
                </div>
                <Badge variant="active">Active Role: {currentRole}</Badge>
              </div>
              <CardTitle className="text-lg">Design System Architectural Parity</CardTitle>
              <CardDescription>
                Every element below is rendered strictly via primitives in <code className="text-xs text-text">@/shared/components/ui/</code>.
                All colors cascade through semantic CSS variables and role-scoped accents. Zero blur shadows.
              </CardDescription>
            </CardHeader>
            <CardContent className="pt-2 text-xs text-text-muted">
              Touch Target Rule: On mobile devices with coarse pointers, all primary controls automatically expand to at least <strong className="text-text">44px</strong> minimum height via <code className="text-xs text-text">touch:min-h-[44px]</code>.
            </CardContent>
          </Card>

          {/* Section 1: Buttons (§5.2, §6.1, §7.0) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">1. Buttons & Action Primitives</h2>
                <p className="text-xs text-text-muted">Citing design.md §5.2, §6.1, §7.0 (Apple 8px radius, micro-compression scale-98, zero blur shadows)</p>
              </div>
              <Badge variant="neutral">§6.1</Badge>
            </div>

            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Variants at Rest</CardTitle>
                  <CardDescription>Default, Secondary, Outline, Destructive, Ghost, Link</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-wrap gap-2.5">
                  <Button variant="default">Primary Action</Button>
                  <Button variant="secondary">Secondary</Button>
                  <Button variant="outline">Outline</Button>
                  <Button variant="destructive">Destructive</Button>
                  <Button variant="ghost">Ghost</Button>
                  <Button variant="link">Link Style</Button>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">States (Mandatory)</CardTitle>
                  <CardDescription>Active, Loading, Disabled, Focus Outline</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-wrap gap-2.5">
                  <Button variant="default" disabled>
                    Disabled Action
                  </Button>
                  <Button variant="secondary" disabled>
                    <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                    Processing...
                  </Button>
                  <Button variant="outline" className="focus-ring">
                    Dual Focus Ring
                  </Button>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Sizes & Tap Targets</CardTitle>
                  <CardDescription>Default (38px / 44px touch), Small (30px), Large, Icon</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-wrap items-center gap-2.5">
                  <Button size="sm">Small (30px)</Button>
                  <Button size="default">Default (38px)</Button>
                  <Button size="lg">Large (44px)</Button>
                  <Button size="icon" aria-label="Icon Action">
                    <Send className="h-4 w-4" />
                  </Button>
                </CardContent>
              </Card>
            </div>
          </section>

          {/* Section 2: Badges & Status Lifecycle (§4.0, §6.4) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">2. Status Badges & Aggregate Lifecycle</h2>
                <p className="text-xs text-text-muted">Citing design.md §4.0, §6.4 (5 semantic variants × 9 aggregate states with 6px circular indicator dot)</p>
              </div>
              <Badge variant="neutral">§4.0 / §6.4</Badge>
            </div>

            <Card>
              <CardHeader>
                <CardTitle className="text-sm">9-State Aggregate Lifecycle Mapping</CardTitle>
                <CardDescription>Domain states mapped to authoritative ink and fill tokens</CardDescription>
              </CardHeader>
              <CardContent className="grid gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-5">
                <div className="space-y-1">
                  <span className="block text-xs font-medium text-text-muted">Draft / Created:</span>
                  <Badge variant="neutral">CREATED</Badge>
                  <Badge variant="neutral">PENDING_QUOTE</Badge>
                  <Badge variant="neutral">DRAFT</Badge>
                </div>
                <div className="space-y-1">
                  <span className="block text-xs font-medium text-text-muted">In Progress / Transit:</span>
                  <Badge variant="active">ASSIGNED</Badge>
                  <Badge variant="active">PICKED_UP</Badge>
                  <Badge variant="active">IN_TRANSIT</Badge>
                  <Badge variant="active">OUT_FOR_DELIVERY</Badge>
                </div>
                <div className="space-y-1">
                  <span className="block text-xs font-medium text-text-muted">Terminal Success:</span>
                  <Badge variant="success">DELIVERED</Badge>
                  <Badge variant="success">PAYMENT_CLEARED</Badge>
                </div>
                <div className="space-y-1">
                  <span className="block text-xs font-medium text-text-muted">Exceptions / Warning:</span>
                  <Badge variant="warning">ATTEMPT_FAILED</Badge>
                  <Badge variant="warning">DELAYED</Badge>
                </div>
                <div className="space-y-1">
                  <span className="block text-xs font-medium text-text-muted">Terminal Error / Cancel:</span>
                  <Badge variant="error">RETURNED</Badge>
                  <Badge variant="error" className="line-through">CANCELLED</Badge>
                </div>
              </CardContent>
            </Card>
          </section>

          {/* Section 3: Form Controls & Inputs (§5.2, §6.2) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">3. Form Controls & Inputs</h2>
                <p className="text-xs text-text-muted">Citing design.md §5.2, §6.2 (Input, Select, Textarea, Switch, Checkbox across Default, Focus, Error, Disabled)</p>
              </div>
              <Badge variant="neutral">§6.2</Badge>
            </div>

            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
              {/* Text Input States */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Input Field States</CardTitle>
                  <CardDescription>Default, Error, and Disabled states</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="demo-input-default">Standard Input</Label>
                    <Input id="demo-input-default" placeholder="e.g. TRK-2026-9082" />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="demo-input-error">Error State</Label>
                    <Input
                      id="demo-input-error"
                      defaultValue="invalid-tracking-id"
                      className="border-status-error-border bg-status-error-bg text-status-error-ink"
                    />
                    <p className="flex items-center gap-1 text-xs text-status-error-ink">
                      <AlertTriangle className="h-3.5 w-3.5" />
                      Tracking number must follow TRK-YYYY-NNNN format.
                    </p>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="demo-input-disabled">Disabled Input</Label>
                    <Input id="demo-input-disabled" disabled value="Locked System Value" />
                  </div>
                </CardContent>
              </Card>

              {/* Select & Textarea */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Select & Textarea</CardTitle>
                  <CardDescription>Dropdown trigger and multiline controls</CardDescription>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="space-y-1.5">
                    <Label>Delivery Service Level</Label>
                    <Select defaultValue="express">
                      <SelectTrigger>
                        <SelectValue placeholder="Select tier" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectLabel>Options</SelectLabel>
                          <SelectItem value="standard">Standard Courier (2-3 Days)</SelectItem>
                          <SelectItem value="express">Express Priority (Same Day)</SelectItem>
                          <SelectItem value="sameday">Immediate Direct Dispatch</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>

                  <div className="space-y-1.5">
                    <Label htmlFor="demo-textarea">Delivery Instructions</Label>
                    <Textarea id="demo-textarea" placeholder="Gate code, reception contact, or handling notes..." />
                  </div>
                </CardContent>
              </Card>

              {/* Toggles & Checkboxes */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Toggles & Selection Controls</CardTitle>
                  <CardDescription>Switch and Checkbox primitives</CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                  <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                      <Label htmlFor="demo-switch" className="text-text">Proof of Delivery Required</Label>
                      <p className="text-xs text-text-muted">Require signature upon handover</p>
                    </div>
                    <Switch
                      id="demo-switch"
                      checked={switchChecked}
                      onCheckedChange={setSwitchChecked}
                    />
                  </div>

                  <Separator />

                  <div className="flex items-start gap-3">
                    <Checkbox
                      id="demo-checkbox"
                      checked={checkboxChecked}
                      onCheckedChange={(checked) => setCheckboxChecked(!!checked)}
                    />
                    <div className="grid gap-1 leading-none">
                      <Label htmlFor="demo-checkbox" className="text-text">
                        Fragile cargo handling protocol
                      </Label>
                      <p className="text-xs text-text-muted">
                        Inspected and sealed in secure transit tote
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3 opacity-50">
                    <Checkbox id="demo-checkbox-disabled" disabled />
                    <Label htmlFor="demo-checkbox-disabled" className="text-xs">
                      Disabled checkbox option
                    </Label>
                  </div>
                </CardContent>
              </Card>
            </div>
          </section>

          {/* Section 4: Data Table & Empty State (§6.5, §7.0) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">4. Data Table & Empty State</h2>
                <p className="text-xs text-text-muted">Citing design.md §6.5, §7.0 (Row hover, tabular nums font-mono, empty state pattern)</p>
              </div>
              <Badge variant="neutral">§6.5</Badge>
            </div>

            <Tabs defaultValue="populated">
              <TabsList className="mb-4">
                <TabsTrigger value="populated">Populated Shipments</TabsTrigger>
                <TabsTrigger value="empty">Empty State Pattern</TabsTrigger>
              </TabsList>

              <TabsContent value="populated">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Tracking ID</TableHead>
                      <TableHead>Destination</TableHead>
                      <TableHead>Service</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Weight (kg)</TableHead>
                      <TableHead className="text-right">Tariff Fee</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    <TableRow>
                      <TableCell className="font-mono font-medium text-primary">TRK-2026-0041</TableCell>
                      <TableCell>Colombo 03, Western Province</TableCell>
                      <TableCell>Express Priority</TableCell>
                      <TableCell><Badge variant="active">IN_TRANSIT</Badge></TableCell>
                      <TableCell className="text-right font-mono tabular-nums">2.450</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">LKR 850.00</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono font-medium text-primary">TRK-2026-0042</TableCell>
                      <TableCell>Kandy City Central Hub</TableCell>
                      <TableCell>Standard</TableCell>
                      <TableCell><Badge variant="success">DELIVERED</Badge></TableCell>
                      <TableCell className="text-right font-mono tabular-nums">0.820</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">LKR 420.00</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell className="font-mono font-medium text-primary">TRK-2026-0043</TableCell>
                      <TableCell>Galle Fort Post Office</TableCell>
                      <TableCell>Same Day</TableCell>
                      <TableCell><Badge variant="warning">DELAYED</Badge></TableCell>
                      <TableCell className="text-right font-mono tabular-nums">5.100</TableCell>
                      <TableCell className="text-right font-mono tabular-nums">LKR 1,650.00</TableCell>
                    </TableRow>
                  </TableBody>
                  <TableCaption>Active live shipments across automated routing zones.</TableCaption>
                </Table>
              </TabsContent>

              <TabsContent value="empty">
                <Card className="flex flex-col items-center justify-center py-12 text-center">
                  <div className="flex h-12 w-12 items-center justify-center rounded-full bg-surface-subtle text-text-muted mb-3">
                    <Package className="h-6 w-6 stroke-[1.5]" />
                  </div>
                  <CardTitle className="text-base font-semibold">No Shipments Found</CardTitle>
                  <CardDescription className="max-w-sm mt-1">
                    There are no parcels matching the current query or filter criteria. Create a new booking or reset filters.
                  </CardDescription>
                  <div className="mt-4 flex gap-2">
                    <Button variant="default" size="sm">Create Booking</Button>
                    <Button variant="outline" size="sm">Reset Filters</Button>
                  </div>
                </Card>
              </TabsContent>
            </Tabs>
          </section>

          {/* Section 5: Overlays, Modals & Toast (§6.0, §6.6, §6.7) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">5. Overlays, Dialogs & Toast Notifications</h2>
                <p className="text-xs text-text-muted">Citing design.md §6.0, §6.6, §6.7 (Zero blur shadows, hairline borders, focus trap, dynamic Sonner sync)</p>
              </div>
              <Badge variant="neutral">§6.6 / §6.7</Badge>
            </div>

            <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-4">
              {/* Dialog Modal */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Dialog (Modal)</CardTitle>
                  <CardDescription>16px radius, backdrop 45%, focus trap</CardDescription>
                </CardHeader>
                <CardContent>
                  <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
                    <DialogTrigger asChild>
                      <Button variant="outline" className="w-full">
                        Open Modal Dialog
                      </Button>
                    </DialogTrigger>
                    <DialogContent>
                      <DialogHeader>
                        <DialogTitle>Confirm Parcel Dispatch</DialogTitle>
                        <DialogDescription>
                          Assigning parcel TRK-2026-0041 to Courier Fleet Unit 09. Once assigned, milestone notifications will be sent to the recipient.
                        </DialogDescription>
                      </DialogHeader>
                      <div className="p-[20px_22px] space-y-3">
                        <div className="space-y-1">
                          <Label>Courier Assigned</Label>
                          <Input readOnly value="Kamal Silva (Unit 09)" />
                        </div>
                      </div>
                      <DialogFooter>
                        <DialogClose asChild>
                          <Button variant="outline">Cancel</Button>
                        </DialogClose>
                        <Button
                          variant="default"
                          onClick={() => {
                            setDialogOpen(false);
                            toast.success('Parcel assigned to Courier Unit 09 successfully');
                          }}
                        >
                          Confirm Dispatch
                        </Button>
                      </DialogFooter>
                    </DialogContent>
                  </Dialog>
                </CardContent>
              </Card>

              {/* Popover & Tooltip */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Popover & Tooltip</CardTitle>
                  <CardDescription>Popover (12px), Tooltip (4px)</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  <Popover>
                    <PopoverTrigger asChild>
                      <Button variant="outline" className="w-full">Open Popover</Button>
                    </PopoverTrigger>
                    <PopoverContent>
                      <div className="space-y-2">
                        <h4 className="font-medium text-sm text-text">Routing Zone</h4>
                        <p className="text-xs text-text-muted">Zone 4B - Western Metro Colombo. Expected SLA 4 hours.</p>
                      </div>
                    </PopoverContent>
                  </Popover>

                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button variant="secondary" className="w-full">Hover for 4px Tooltip</Button>
                    </TooltipTrigger>
                    <TooltipContent>
                      <span>Carrier Hub Transit Point (4px radius)</span>
                    </TooltipContent>
                  </Tooltip>
                </CardContent>
              </Card>

              {/* Dropdown Menu */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Dropdown Menu</CardTitle>
                  <CardDescription>12px radius, zero shadow</CardDescription>
                </CardHeader>
                <CardContent>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="outline" className="w-full">
                        Action Menu
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent>
                      <DropdownMenuLabel>Shipment Actions</DropdownMenuLabel>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem onClick={() => toast.info('Printing shipping label...')}>
                        Print Barcode Label
                      </DropdownMenuItem>
                      <DropdownMenuItem onClick={() => toast.info('Recalculating tariff...')}>
                        Re-estimate Tariff Fee
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-status-error-ink focus:bg-status-error-bg"
                        onClick={() => toast.error('Shipment booking cancelled')}
                      >
                        Cancel Booking
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </CardContent>
              </Card>

              {/* Toast Triggers */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Sonner Toast Triggers</CardTitle>
                  <CardDescription>Dynamic light/dark theme sync</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => toast.success('Shipment TRK-2026-0041 marked Delivered')}
                  >
                    Trigger Success Toast
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => toast.warning('Delivery attempt failed: Recipient unavailable')}
                  >
                    Trigger Warning Toast
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => toast.error('Connection dropped: RabbitMQ broker unreachable')}
                  >
                    Trigger Error Toast
                  </Button>
                </CardContent>
              </Card>
            </div>
          </section>

          {/* Section 6: Layout, Accordion, Skeleton, Avatar (§6.0) */}
          <section className="space-y-4">
            <div className="flex items-center justify-between border-b border-border pb-2">
              <div>
                <h2 className="text-base font-semibold text-text">6. Layout, Skeleton & Accordion</h2>
                <p className="text-xs text-text-muted">Citing design.md §6.0, §7.0 (Shimmer animation sweep, circular avatar, collapsible accordion)</p>
              </div>
              <Badge variant="neutral">§6.0</Badge>
            </div>

            <div className="grid gap-6 md:grid-cols-3">
              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Accordion Collapsible</CardTitle>
                  <CardDescription>Expandable logistics policy FAQ</CardDescription>
                </CardHeader>
                <CardContent>
                  <Accordion type="single" collapsible defaultValue="item-1">
                    <AccordionItem value="item-1">
                      <AccordionTrigger>What is the weight limit?</AccordionTrigger>
                      <AccordionContent>
                        Standard courier parcels accept up to 30kg. Shipments exceeding 30kg require freight dispatch clearance.
                      </AccordionContent>
                    </AccordionItem>
                    <AccordionItem value="item-2">
                      <AccordionTrigger>How does cash-on-delivery work?</AccordionTrigger>
                      <AccordionContent>
                        Couriers collect exact cash amounts and remit to merchant tenant balance within 24 hours of delivery.
                      </AccordionContent>
                    </AccordionItem>
                  </Accordion>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Shimmer Skeleton</CardTitle>
                  <CardDescription>Loading placeholder with 1.6s sweep</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  <div className="flex items-center gap-3">
                    <Skeleton className="h-10 w-10 rounded-full" />
                    <div className="space-y-1.5 flex-1">
                      <Skeleton className="h-4 w-3/4" />
                      <Skeleton className="h-3 w-1/2" />
                    </div>
                  </div>
                  <Skeleton className="h-20 w-full" />
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm">Avatar & Profile Primitives</CardTitle>
                  <CardDescription>9999px circular radius</CardDescription>
                </CardHeader>
                <CardContent className="flex items-center gap-4">
                  <Avatar>
                    <AvatarImage src="/avatar-demo.png" alt="Courier Driver" />
                    <AvatarFallback>KS</AvatarFallback>
                  </Avatar>
                  <div>
                    <p className="text-sm font-semibold text-text">Kamal Silva</p>
                    <p className="text-xs text-text-muted">Fleet Unit 09 • Zone 4B Courier</p>
                  </div>
                </CardContent>
                <CardFooter className="pt-0">
                  <p className="text-xs text-text-muted">CardFooter structural boundary</p>
                </CardFooter>
              </Card>

              <Card className="md:col-span-3">
                <CardHeader>
                  <CardTitle className="text-sm">Scroll Area Primitive</CardTitle>
                  <CardDescription>Custom hairline scrollbar</CardDescription>
                </CardHeader>
                <CardContent>
                  <ScrollArea className="h-28 w-full rounded-[8px] border border-border p-3">
                    <div className="space-y-2 text-xs text-text-muted">
                      <p>• Zone 1: Colombo Central Hub — Capacity 85% — Status: Operational</p>
                      <p>• Zone 2: Gampaha Dispatch Node — Capacity 62% — Status: Operational</p>
                      <p>• Zone 3: Kalutara Coastal Transit — Capacity 45% — Status: Operational</p>
                      <p>• Zone 4: Kandy Central Sorting — Capacity 91% — Status: Operational</p>
                      <p>• Zone 5: Galle Southern Terminal — Capacity 38% — Status: Operational</p>
                    </div>
                  </ScrollArea>
                </CardContent>
              </Card>
            </div>
          </section>
        </main>
      </div>
    </TooltipProvider>
  );
};

export default DesignSystemPage;
