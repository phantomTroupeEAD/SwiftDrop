// Idempotency-Key rules (Doc 2 3.3): one key per user action, reused on retry
// with the same input, replaced when the input changes, dropped after success.
export class IdempotentAction<I> {
  private key: string | null = null;
  private lastInput: string | null = null;
  constructor(private readonly newKey: () => string = () => crypto.randomUUID()) {}

  keyFor(input: I): string {
    const s = JSON.stringify(input);
    if (this.key === null || s !== this.lastInput) {
      this.key = this.newKey();
      this.lastInput = s;
    }
    return this.key;
  }

  succeeded(): void {
    this.key = null;
    this.lastInput = null;
  }
}
