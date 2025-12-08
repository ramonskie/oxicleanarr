/**
 * Client-side error logging service
 * Logs errors to console and can be extended to send to remote logging service
 */

export interface ErrorLogEntry {
  timestamp: Date;
  message: string;
  stack?: string;
  context?: Record<string, any>;
  level: 'error' | 'warn' | 'info';
}

class ErrorLogger {
  private logs: ErrorLogEntry[] = [];
  private maxLogs = 100; // Keep only the last 100 logs in memory

  /**
   * Log an error
   */
  error(message: string, error?: Error | unknown, context?: Record<string, any>): void {
    const entry: ErrorLogEntry = {
      timestamp: new Date(),
      message,
      stack: error instanceof Error ? error.stack : undefined,
      context: {
        ...context,
        userAgent: navigator.userAgent,
        url: window.location.href,
      },
      level: 'error',
    };

    this.addLog(entry);
    
    // Log to console
    console.error(`[ErrorLogger] ${message}`, error, context);

    // TODO: Send to remote logging service (e.g., Sentry, LogRocket, custom endpoint)
    // this.sendToRemote(entry);
  }

  /**
   * Log a warning
   */
  warn(message: string, context?: Record<string, any>): void {
    const entry: ErrorLogEntry = {
      timestamp: new Date(),
      message,
      context,
      level: 'warn',
    };

    this.addLog(entry);
    console.warn(`[ErrorLogger] ${message}`, context);
  }

  /**
   * Log info
   */
  info(message: string, context?: Record<string, any>): void {
    const entry: ErrorLogEntry = {
      timestamp: new Date(),
      message,
      context,
      level: 'info',
    };

    this.addLog(entry);
    console.info(`[ErrorLogger] ${message}`, context);
  }

  /**
   * Add log to in-memory storage
   */
  private addLog(entry: ErrorLogEntry): void {
    this.logs.push(entry);
    
    // Keep only the last N logs
    if (this.logs.length > this.maxLogs) {
      this.logs = this.logs.slice(-this.maxLogs);
    }
  }

  /**
   * Get recent logs (useful for debugging)
   */
  getRecentLogs(count = 10): ErrorLogEntry[] {
    return this.logs.slice(-count);
  }

  /**
   * Clear all logs
   */
  clearLogs(): void {
    this.logs = [];
  }

  /**
   * Send error to remote logging service
   * Override this method to implement custom remote logging
   */
  // @ts-ignore - Unused method kept for future implementation
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  private async sendToRemote(entry: ErrorLogEntry): Promise<void> {
    // Skip in development
    if (import.meta.env.DEV) {
      return;
    }

    try {
      // TODO: Implement remote logging
      // Example: await fetch('/api/client-errors', {
      //   method: 'POST',
      //   headers: { 'Content-Type': 'application/json' },
      //   body: JSON.stringify(entry),
      // });
    } catch (err) {
      // Silently fail - don't want logging errors to break the app
      console.error('Failed to send error to remote service:', err);
    }
  }
}

// Export singleton instance
export const errorLogger = new ErrorLogger();

// Attach to window for debugging in development
if (import.meta.env.DEV) {
  (window as any).errorLogger = errorLogger;
}
