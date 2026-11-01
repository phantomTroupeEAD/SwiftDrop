// Single-flight refresh (Doc 2 F-02): many 401s share one refresh call,
// because refresh tokens are single use and reuse logs the user out everywhere.
export class RefreshLock {
  private pending: Promise<string | null> | null = null;
  constructor(private readonly doRefresh: () => Promise<string | null>) {}

  run(): Promise<string | null> {
    if (!this.pending) {
      this.pending = this.doRefresh().finally(() => {
        this.pending = null;
      });
    }
    return this.pending;
  }
}
