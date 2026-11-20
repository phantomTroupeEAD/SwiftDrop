import { Page } from '@playwright/test';

export interface HudOptions {
  step: number;
  totalSteps?: number;
  title: string;
  subtitle?: string;
  badge?: string;
  durationMs?: number;
}

/**
 * Injects or updates an Apple-grade presenter HUD banner at the top of the browser page.
 * Provides clear visual commentary for evaluator review and video recordings.
 */
export async function showHud(page: Page, options: HudOptions): Promise<void> {
  const total = options.totalSteps || 16;
  const progressPercent = Math.min(100, Math.round((options.step / total) * 100));
  const badgeText = options.badge || `STEP ${String(options.step).padStart(2, '0')} / ${String(total).padStart(2, '0')}`;

  await page.evaluate(
    ({ step, total, progress, badge, title, subtitle }) => {
      let hud = document.getElementById('swiftdrop-evaluator-hud');
      if (!hud) {
        hud = document.createElement('div');
        hud.id = 'swiftdrop-evaluator-hud';
        hud.style.position = 'fixed';
        hud.style.top = '12px';
        hud.style.left = '50%';
        hud.style.transform = 'translateX(-50%)';
        hud.style.zIndex = '9999999';
        hud.style.width = 'min(900px, 92vw)';
        hud.style.padding = '12px 20px';
        hud.style.borderRadius = '16px';
        hud.style.background = 'rgba(15, 23, 42, 0.92)';
        hud.style.backdropFilter = 'blur(16px) saturate(180%)';
        hud.style.webkitBackdropFilter = 'blur(16px) saturate(180%)';
        hud.style.border = '1px solid rgba(255, 255, 255, 0.12)';
        hud.style.boxShadow = '0 12px 36px -4px rgba(0, 0, 0, 0.4), 0 4px 12px rgba(0, 0, 0, 0.2)';
        hud.style.color = '#F8FAFC';
        hud.style.fontFamily = '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif';
        hud.style.pointerEvents = 'none';
        hud.style.transition = 'all 300ms cubic-bezier(0.16, 1, 0.3, 1)';
        document.body.appendChild(hud);
      }

      hud.innerHTML = `
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 6px;">
          <div style="display: flex; align-items: center; gap: 10px;">
            <span style="background: linear-gradient(135deg, #2563EB, #3B82F6); color: #FFFFFF; font-weight: 700; font-size: 11px; letter-spacing: 0.08em; padding: 4px 10px; border-radius: 9999px; text-transform: uppercase; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.4);">
              ${badge}
            </span>
            <span style="font-weight: 700; font-size: 15px; color: #FFFFFF; letter-spacing: -0.01em;">
              ${title}
            </span>
          </div>
          <span style="font-size: 12px; color: #94A3B8; font-weight: 500; font-variant-numeric: tabular-nums;">
            ${progress}% Complete
          </span>
        </div>
        ${subtitle ? `<div style="font-size: 13px; color: #CBD5E1; line-height: 1.4; margin-bottom: 8px;">${subtitle}</div>` : ''}
        <div style="width: 100%; height: 4px; background: rgba(255, 255, 255, 0.1); border-radius: 9999px; overflow: hidden;">
          <div style="width: ${progress}%; height: 100%; background: linear-gradient(90deg, #3B82F6, #10B981); border-radius: 9999px; transition: width 400ms ease-out;"></div>
        </div>
      `;
    },
    {
      step: options.step,
      total,
      progress: progressPercent,
      badge: badgeText,
      title: options.title,
      subtitle: options.subtitle || '',
    }
  );

  // Brief pause so evaluator and video capture the HUD update clearly
  const waitMs = options.durationMs !== undefined ? options.durationMs : 1000;
  if (waitMs > 0) {
    await page.waitForTimeout(waitMs);
  }
}
