'use client';

import { useState, useEffect, useCallback } from 'react';
import { useAuth } from '@/lib/auth';
import { activityAPI } from '@/lib/api';
import { Activity } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { toast } from 'sonner';
import { 
  Bell, 
  Upload, 
  Download, 
  Trash2, 
  Share2, 
  FolderPlus, 
  Users, 
  CheckCheck,
  Loader2
} from 'lucide-react';

function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffInSeconds = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffInSeconds < 60) {
    return 'just now';
  }

  const diffInMinutes = Math.floor(diffInSeconds / 60);
  if (diffInMinutes < 60) {
    return `${diffInMinutes} minute${diffInMinutes > 1 ? 's' : ''} ago`;
  }

  const diffInHours = Math.floor(diffInMinutes / 60);
  if (diffInHours < 24) {
    return `${diffInHours} hour${diffInHours > 1 ? 's' : ''} ago`;
  }

  const diffInDays = Math.floor(diffInHours / 24);
  if (diffInDays < 7) {
    return `${diffInDays} day${diffInDays > 1 ? 's' : ''} ago`;
  }

  const diffInWeeks = Math.floor(diffInDays / 7);
  if (diffInWeeks < 4) {
    return `${diffInWeeks} week${diffInWeeks > 1 ? 's' : ''} ago`;
  }

  return date.toLocaleDateString();
}

function getActivityIcon(action: string) {
  switch (action) {
    case 'file.upload':
      return <Upload className="h-5 w-5 text-blue-500" />;
    case 'file.download':
      return <Download className="h-5 w-5 text-green-500" />;
    case 'file.delete':
      return <Trash2 className="h-5 w-5 text-red-500" />;
    case 'share.create':
    case 'share.delete':
      return <Share2 className="h-5 w-5 text-purple-500" />;
    case 'folder.create':
      return <FolderPlus className="h-5 w-5 text-yellow-500" />;
    case 'group.member_add':
    case 'group.member_remove':
      return <Users className="h-5 w-5 text-indigo-500" />;
    default:
      return <Bell className="h-5 w-5 text-gray-500" />;
  }
}

export default function ActivityPage() {
  const { user } = useAuth();
  const [activities, setActivities] = useState<Activity[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [page, setPage] = useState(1);
  const [hasMore, setHasMore] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [markingAllRead, setMarkingAllRead] = useState(false);

  const fetchActivities = useCallback(async (pageNum: number, append: boolean = false) => {
    try {
      const res = await activityAPI.list(pageNum, 20);
      if (res.success) {
        if (append) {
          setActivities(prev => [...prev, ...res.data]);
        } else {
          setActivities(res.data);
        }
        
        if (res.pagination) {
          setHasMore(pageNum < res.pagination.totalPages);
        } else {
          setHasMore(res.data.length === 20);
        }
      }
    } catch (error) {
      console.error('Failed to fetch activities:', error);
      toast.error('Failed to load activities');
    } finally {
      setIsLoading(false);
      setLoadingMore(false);
    }
  }, []);

  useEffect(() => {
    if (user) {
      fetchActivities(1);
    }
  }, [user, fetchActivities]);

  const handleLoadMore = () => {
    setLoadingMore(true);
    const nextPage = page + 1;
    setPage(nextPage);
    fetchActivities(nextPage, true);
  };

  const handleMarkRead = async (id: string, isRead: boolean) => {
    if (isRead) return;

    setActivities(prev => prev.map(a => 
      a.id === id ? { ...a, isRead: true } : a
    ));

    try {
      await activityAPI.markRead(id);
    } catch (error) {
      console.error('Failed to mark activity as read:', error);
      setActivities(prev => prev.map(a => 
        a.id === id ? { ...a, isRead: false } : a
      ));
      toast.error('Failed to mark as read');
    }
  };

  const handleMarkAllRead = async () => {
    setMarkingAllRead(true);
    
    const previousActivities = [...activities];
    setActivities(prev => prev.map(a => ({ ...a, isRead: true })));

    try {
      await activityAPI.markAllRead();
      toast.success('All activities marked as read');
    } catch (error) {
      console.error('Failed to mark all as read:', error);
      setActivities(previousActivities);
      toast.error('Failed to mark all as read');
    } finally {
      setMarkingAllRead(false);
    }
  };

  if (!user) {
    return null;
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="flex items-center justify-between">
        <div className="space-y-1">
          <h1 className="text-3xl font-bold">Activity</h1>
          <p className="text-muted-foreground">
            Recent activity on your files and shares
          </p>
        </div>
        <Button 
          variant="outline" 
          onClick={handleMarkAllRead}
          disabled={markingAllRead || activities.every(a => a.isRead) || activities.length === 0}
        >
          {markingAllRead ? (
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
          ) : (
            <CheckCheck className="mr-2 h-4 w-4" />
          )}
          Mark all as read
        </Button>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Timeline</CardTitle>
          <CardDescription>Your latest notifications and events</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          ) : activities.length === 0 ? (
            <div className="text-center py-12 text-muted-foreground">
              <Bell className="h-12 w-12 mx-auto mb-4 opacity-20" />
              <p>No recent activity</p>
            </div>
          ) : (
            <div className="space-y-1">
              {activities.map((activity, index) => (
                <div key={activity.id}>
                  <div 
                    role="button"
                    tabIndex={0}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        handleMarkRead(activity.id, activity.isRead);
                      }
                    }}
                    className={`flex gap-4 p-4 rounded-lg transition-colors cursor-pointer hover:bg-muted/50 ${!activity.isRead ? 'bg-muted/20' : ''}`}
                    onClick={() => handleMarkRead(activity.id, activity.isRead)}
                  >
                    <div className="mt-1">
                      {getActivityIcon(activity.action)}
                    </div>
                    <div className="flex-1 space-y-1">
                      <div className="flex items-start justify-between gap-2">
                        <p className={`text-sm ${!activity.isRead ? 'font-medium' : ''}`}>
                          {activity.message}
                        </p>
                        {!activity.isRead && (
                          <span className="h-2 w-2 rounded-full bg-blue-500 mt-1.5 flex-shrink-0" />
                        )}
                      </div>
                      <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <span>{formatRelativeTime(activity.createdAt)}</span>
                        {activity.resourceName && (
                          <>
                            <span>•</span>
                            <Badge variant="outline" className="text-xs font-normal">
                              {activity.resourceName}
                            </Badge>
                          </>
                        )}
                      </div>
                    </div>
                  </div>
                  {index < activities.length - 1 && <Separator className="my-1" />}
                </div>
              ))}

              {hasMore && (
                <div className="pt-4 flex justify-center">
                  <Button 
                    variant="ghost" 
                    onClick={handleLoadMore}
                    disabled={loadingMore}
                  >
                    {loadingMore ? (
                      <>
                        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                        Loading...
                      </>
                    ) : (
                      'Load more'
                    )}
                  </Button>
                </div>
              )}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
