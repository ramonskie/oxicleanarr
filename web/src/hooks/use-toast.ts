import { useCallback } from 'react';

export interface Toast {
  title: string;
  description?: string;
  variant?: 'default' | 'destructive';
}

export function useToast() {
  const toast = useCallback(({ title, description, variant = 'default' }: Toast) => {
    // For now, use a simple alert-style notification
    // This can be replaced with a proper toast component later
    const message = description ? `${title}: ${description}` : title;
    
    if (variant === 'destructive') {
      console.error(message);
      alert(`Error: ${message}`);
    } else {
      console.log(message);
      // Could add a proper toast notification here
      // For now, silently succeed (user will see the UI update)
    }
  }, []);

  return { toast };
}
