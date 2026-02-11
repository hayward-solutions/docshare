'use client';

import { useState, useEffect, useCallback } from 'react';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from '@/components/ui/tabs';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Share2, User as UserIcon, Users, Loader2, Trash2 } from 'lucide-react';
import { apiMethods } from '@/lib/api';
import { toast } from 'sonner';
import { User, Group, Share } from '@/lib/types';

interface ShareDialogProps {
  fileId: string;
  fileName: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function ShareDialog({ fileId, fileName, open, onOpenChange }: ShareDialogProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isControlled = open !== undefined;
  const dialogOpen = isControlled ? open : internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;
  const [activeTab, setActiveTab] = useState('share');
  const [searchQuery, setSearchQuery] = useState('');
  const [users, setUsers] = useState<User[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [shares, setShares] = useState<Share[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>('');
  const [selectedGroup, setSelectedGroup] = useState<string>('');
  const [permission, setPermission] = useState<'view' | 'download' | 'edit'>('view');
  const [isLoading, setIsLoading] = useState(false);

  const fetchShares = useCallback(async () => {
    try {
      const res = await apiMethods.get<Share[]>(`/api/files/${fileId}/shares`);
      if (res.success) {
        setShares(res.data);
      }
    } catch (error) {
      console.error('Failed to fetch shares', error);
    }
  }, [fileId]);

  const fetchGroups = useCallback(async () => {
    try {
      const res = await apiMethods.get<Group[]>('/api/groups');
      if (res.success) {
        setGroups(res.data);
      }
    } catch (error) {
      console.error('Failed to fetch groups', error);
    }
  }, []);

  useEffect(() => {
    if (open) {
      fetchShares();
      fetchGroups();
    }
  }, [open, fetchShares, fetchGroups]);

   useEffect(() => {
     const searchUsers = async () => {
       if (searchQuery.length < 2) {
         setUsers([]);
         return;
       }
       try {
         const res = await apiMethods.get<User[]>('/api/users/search', { search: searchQuery, limit: 5 });
         
          const data = res as { success: boolean; data?: { users?: User[] } | User[] };
         if (data.success && data.data) {
            if (Array.isArray(data.data)) {
              setUsers(data.data);
            } else if (data.data.users) {
              setUsers(data.data.users);
            }
         }
       } catch (error) {
         console.error('Failed to search users', error);
       }
     };

     const timeoutId = setTimeout(searchUsers, 300);
     return () => clearTimeout(timeoutId);
   }, [searchQuery]);

  const handleShare = async () => {
    if (!selectedUser && !selectedGroup) return;
    
    setIsLoading(true);
    try {
      await apiMethods.post(`/api/files/${fileId}/share`, {
        userID: selectedUser || undefined,
        groupID: selectedGroup || undefined,
        permission
      });
      toast.success('File shared successfully');
      setSearchQuery('');
      setSelectedUser('');
      setSelectedGroup('');
      fetchShares();
      setActiveTab('permissions');
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to share file');
    } finally {
      setIsLoading(false);
    }
  };

  const handleRemoveShare = async (shareId: string) => {
    try {
      await apiMethods.delete(`/api/shares/${shareId}`);
      toast.success('Share removed');
      fetchShares();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to remove share');
    }
  };

  const handleUpdatePermission = async (shareId: string, newPermission: string) => {
    try {
      await apiMethods.put(`/api/shares/${shareId}`, { permission: newPermission });
      toast.success('Permission updated');
      fetchShares();
    } catch (error: unknown) {
      toast.error(error instanceof Error ? error.message : 'Failed to update permission');
    }
  };

  return (
    <Dialog open={dialogOpen} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <Share2 className="mr-2 h-4 w-4" />
          Share
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Share &quot;{fileName}&quot;</DialogTitle>
          <DialogDescription>
            Share this item with users or groups.
          </DialogDescription>
        </DialogHeader>
        
        <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
          <TabsList className="grid w-full grid-cols-2">
            <TabsTrigger value="share">Share</TabsTrigger>
            <TabsTrigger value="permissions">Manage Access ({shares.length})</TabsTrigger>
          </TabsList>
          
          <TabsContent value="share" className="space-y-4 py-4">
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Share with User</Label>
                <div className="relative">
                  <UserIcon className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                  <Input
                    placeholder="Search by name or email..."
                    className="pl-9"
                    value={searchQuery}
                    onChange={(e) => {
                      setSearchQuery(e.target.value);
                      setSelectedGroup('');
                    }}
                  />
                </div>
                {users.length > 0 && searchQuery && (
                  <div className="rounded-md border bg-popover p-1 text-popover-foreground shadow-md">
                    {users.map((u) => (
                      <button
                        key={u.id}
                        type="button"
                        className={`flex w-full cursor-pointer items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent hover:text-accent-foreground ${selectedUser === u.id ? 'bg-accent' : ''}`}
                        onClick={() => {
                          setSelectedUser(u.id);
                          setSearchQuery(`${u.firstName} ${u.lastName} (${u.email})`);
                          setUsers([]);
                        }}
                      >
                        <Avatar className="h-6 w-6">
                          <AvatarImage src={u.avatarURL} />
                          <AvatarFallback>{u.firstName[0]}</AvatarFallback>
                        </Avatar>
                        <span>{u.firstName} {u.lastName}</span>
                        <span className="text-xs text-muted-foreground ml-auto">{u.email}</span>
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div className="relative flex items-center py-2">
                <div className="grow border-t border-slate-200"></div>
                <span className="mx-4 shrink-0 text-slate-400 text-xs">OR</span>
                <div className="grow border-t border-slate-200"></div>
              </div>

              <div className="space-y-2">
                <Label>Share with Group</Label>
                <Select 
                  value={selectedGroup} 
                  onValueChange={(val) => {
                    setSelectedGroup(val);
                    setSelectedUser('');
                    setSearchQuery('');
                  }}
                >
                  <SelectTrigger>
                    <SelectValue placeholder="Select a group" />
                  </SelectTrigger>
                  <SelectContent>
                    {groups.map((group) => (
                      <SelectItem key={group.id} value={group.id}>
                        {group.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label>Permission</Label>
                <Select value={permission} onValueChange={(val: 'view' | 'download' | 'edit') => setPermission(val)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="view">View Only</SelectItem>
                    <SelectItem value="download">View & Download</SelectItem>
                    <SelectItem value="edit">Edit (Rename/Delete)</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <Button 
                className="w-full" 
                onClick={handleShare}
                disabled={(!selectedUser && !selectedGroup) || isLoading}
              >
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Share
              </Button>
            </div>
          </TabsContent>
          
          <TabsContent value="permissions" className="py-4">
            <ScrollArea className="h-[300px] pr-4">
              <div className="space-y-4">
                {shares.length === 0 ? (
                  <div className="text-center text-sm text-muted-foreground py-8">
                    No one has access to this item yet.
                  </div>
                ) : (
                  shares.map((share) => (
                    <div key={share.id} className="flex items-center justify-between space-x-4 rounded-lg border p-3">
                      <div className="flex items-center space-x-3">
                        <Avatar className="h-8 w-8">
                          {share.sharedWithUser ? (
                            <>
                              <AvatarImage src={share.sharedWithUser.avatarURL} />
                              <AvatarFallback>{share.sharedWithUser.firstName[0]}</AvatarFallback>
                            </>
                          ) : (
                            <AvatarFallback><Users className="h-4 w-4" /></AvatarFallback>
                          )}
                        </Avatar>
                        <div>
                          <p className="text-sm font-medium leading-none">
                            {share.sharedWithUser 
                              ? `${share.sharedWithUser.firstName} ${share.sharedWithUser.lastName}`
                              : share.sharedWithGroup?.name || 'Unknown Group'}
                          </p>
                          <p className="text-xs text-muted-foreground">
                            {share.sharedWithUser ? share.sharedWithUser.email : 'Group'}
                          </p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        <Select 
                          defaultValue={share.permission} 
                          onValueChange={(val) => handleUpdatePermission(share.id, val)}
                        >
                          <SelectTrigger className="h-8 w-[110px] text-xs">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="view">View</SelectItem>
                            <SelectItem value="download">Download</SelectItem>
                            <SelectItem value="edit">Edit</SelectItem>
                          </SelectContent>
                        </Select>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-8 w-8 text-red-500 hover:text-red-600 hover:bg-red-50"
                          onClick={() => handleRemoveShare(share.id)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </div>
                  ))
                )}
              </div>
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
