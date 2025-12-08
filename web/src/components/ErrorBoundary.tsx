import { Component, type ErrorInfo, type ReactNode } from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { errorLogger } from '@/lib/error-logger';

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
  errorInfo: ErrorInfo | null;
}

class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = {
      hasError: false,
      error: null,
      errorInfo: null,
    };
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return {
      hasError: true,
      error,
    };
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo): void {
    // Log error to console in development
    console.error('ErrorBoundary caught an error:', error, errorInfo);
    
    // Store error info in state
    this.setState({
      error,
      errorInfo,
    });

    // Log to error logging service
    errorLogger.error('React component error caught by ErrorBoundary', error, {
      componentStack: errorInfo.componentStack,
    });
  }

  handleReset = (): void => {
    this.setState({
      hasError: false,
      error: null,
      errorInfo: null,
    });
  };

  render(): ReactNode {
    if (this.state.hasError) {
      // Use custom fallback if provided
      if (this.props.fallback) {
        return this.props.fallback;
      }

      // Default error UI
      return (
        <div className="min-h-screen bg-[#0d0d0d] flex items-center justify-center p-4">
          <div className="max-w-2xl w-full bg-[#1a1a1a] border border-[#333] rounded-lg p-8">
            <div className="flex items-center gap-4 mb-6">
              <div className="h-12 w-12 rounded-full bg-red-900/20 flex items-center justify-center flex-shrink-0">
                <AlertTriangle className="h-6 w-6 text-red-500" />
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white mb-1">
                  Something went wrong
                </h1>
                <p className="text-gray-400">
                  An unexpected error occurred in the application
                </p>
              </div>
            </div>

            {/* Error Details (Development) */}
            {import.meta.env.DEV && this.state.error && (
              <div className="mb-6 p-4 bg-[#262626] border border-[#444] rounded-md">
                <p className="text-sm font-semibold text-red-400 mb-2">
                  Error Details:
                </p>
                <p className="text-sm text-gray-300 mb-2 font-mono">
                  {this.state.error.toString()}
                </p>
                {this.state.errorInfo && (
                  <details className="text-xs text-gray-500 mt-2">
                    <summary className="cursor-pointer hover:text-gray-400">
                      Stack trace
                    </summary>
                    <pre className="mt-2 overflow-auto max-h-64 p-2 bg-[#1a1a1a] rounded">
                      {this.state.errorInfo.componentStack}
                    </pre>
                  </details>
                )}
              </div>
            )}

            {/* Actions */}
            <div className="flex gap-3">
              <Button
                onClick={this.handleReset}
                className="bg-blue-600 hover:bg-blue-700 text-white"
              >
                <RefreshCw className="h-4 w-4 mr-2" />
                Try Again
              </Button>
              <Button
                onClick={() => window.location.href = '/'}
                variant="outline"
                className="bg-[#262626] border-[#444] text-gray-300 hover:bg-[#333] hover:text-white"
              >
                Go to Dashboard
              </Button>
            </div>

            {/* Help Text */}
            <div className="mt-6 pt-6 border-t border-[#333]">
              <p className="text-sm text-gray-400">
                If this problem persists, try:
              </p>
              <ul className="mt-2 text-sm text-gray-500 list-disc list-inside space-y-1">
                <li>Refreshing the page</li>
                <li>Clearing your browser cache</li>
                <li>Checking the browser console for more details</li>
                <li>Reporting the issue on GitHub</li>
              </ul>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
