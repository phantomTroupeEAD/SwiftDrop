import { Page, TestInfo } from '@playwright/test';
import * as path from 'path';
import * as fs from 'fs';

/**
 * Academic Grading & Evidence Capture Helper
 * Automatically saves milestone screenshots and attaches traces for grading evaluations.
 *
 * Owned by: Member 10 (Cross-Workflow Quality Coordination)
 */
export class EvidenceCollector {
  private outputDir: string;

  constructor(private testInfo: TestInfo) {
    this.outputDir = path.resolve(process.cwd(), 'artifacts', 'evidence');
    if (!fs.existsSync(this.outputDir)) {
      fs.mkdirSync(this.outputDir, { recursive: true });
    }
  }

  /**
   * Captures a full-page milestone screenshot and attaches it to the test report.
   */
  async captureMilestone(page: Page, milestoneName: string): Promise<string> {
    const sanitizedName = milestoneName.replace(/[^a-zA-Z0-9_-]/g, '_').toLowerCase();
    const filename = `${this.testInfo.title.replace(/[^a-zA-Z0-9_-]/g, '_')}_${sanitizedName}_${Date.now()}.png`;
    const filepath = path.join(this.outputDir, filename);

    await page.screenshot({
      path: filepath,
      fullPage: true,
    });

    await this.testInfo.attach(milestoneName, {
      path: filepath,
      contentType: 'image/png',
    });

    return filepath;
  }
}
