import { toast } from 'sonner';
import { useActivity } from '@/contexts/activity-context';

export function useActivityToast() {
  const { refreshActivityCount } = useActivity();

  const successWithRefresh = (message: string) => {
    toast.success(message);
    refreshActivityCount();
  };

  return {
    successWithRefresh,
    refreshActivityCount,
  };
}
