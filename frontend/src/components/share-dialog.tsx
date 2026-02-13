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
import { Share2, User as UserIcon, Users, Loader2, Trash2, Globe, LogIn, Check, Link } from 'lucide-react';
import { apiMethods } from '@/lib/api';
import { toast } from 'sonner';
import { User, Group, Share, ShareType } from '@/lib/types';

function getPublicLink(fileId: string): string {
  const origin = typeof window !== 'undefined' ? window.location.origin : '';
  return `${origin}/shared/${fileId}`;
}

function CopyLinkButton({ fileId }: { fileId: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(getPublicLink(fileId));
    setCopied(true);
    toast.success('Link copied to clipboard');
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Button variant="outline" size="sm" className="h-7 text-xs" onClick={handleCopy}>
      {copied ? <Check className="mr-1.5 h-3 w-3 text-green-600" /> : <Link className="mr-1.5 h-3 w-3" />}
      {copied ? 'Copied' : 'Copy link'}
    </Button>
  );
}

interface ShareDialogProps {
  fileId?: string;
  fileIds?: string[];
  fileName: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

export function ShareDialog({ fileId, fileIds, fileName, open, onOpenChange }: ShareDialogProps) {
  const resolvedIds = fileIds ?? (fileId ? [fileId] : []);
  const isBulk = resolvedIds.length > 1;
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
  const [shareType, setShareType] = useState<ShareType>('private');
  const [permission, setPermission] = useState<'view' | 'download' | 'edit'>('view');
  const [isLoading, setIsLoading] = useState(false);

  const fetchShares = useCallback(async () => {
    if (isBulk) return;
    const id = resolvedIds[0];
    if (!id) return;
    try {
      const res = await apiMethods.get<Share[]>(`/api/files/${id}/shares`);
      if (res.success && res.data) {
        setShares(res.data);
      }
    } catch (error) {
      console.error('Failed to fetch shares', error);
    }
  }, [isBulk, resolvedIds]);

  const fetchGroups = useCallback(async () => {
    try {
      const res = await apiMethods.get<Group[]>('/api/groups');
      if (res.success && res.data) {
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
    if (shareType === 'private' && !selectedUser && !selectedGroup) return;
    
    setIsLoading(true);
    try {
      const body: Record<string, unknown> = {
        permission,
        shareType,
      };
      if (shareType === 'private') {
        body.userID = selectedUser || undefined;
        body.groupID = selectedGroup || undefined;
      }

      await Promise.all(
        resolvedIds.map((id) =>
          apiMethods.post(`/api/files/${id}/share`, body),
        ),
      );
      const label = shareType === 'public_anyone'
        ? 'Public link created'
        : shareType === 'public_logged_in'
        ? 'Public (logged in) link created'
        : isBulk ? `${resolvedIds.length} items shared` : 'File shared successfully';
      toast.success(label);
      setSearchQuery('');
      setSelectedUser('');
      setSelectedGroup('');
      setShareType('private');
      if (!isBulk) {
        fetchShares();
        setActiveTab('permissions');
      } else {
        onOpenChange?.(false);
        setInternalOpen(false);
      }
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
        
        <Tabs value={isBulk ? 'share' : activeTab} onValueChange={setActiveTab} className="w-full">
          {!isBulk && (
            <TabsList className="grid w-full grid-cols-2">
              <TabsTrigger value="share">Share</TabsTrigger>
              <TabsTrigger value="permissions">Manage Access ({shares.length})</TabsTrigger>
            </TabsList>
          )}
          
          <TabsContent value="share" className="space-y-4 py-4">
            <div className="space-y-4">
              <div className="space-y-2">
                <Label>Share Type</Label>
                <div className="grid grid-cols-3 gap-2">
                  <button
                    type="button"
                    className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-xs transition-colors ${shareType === 'private' ? 'border-blue-500 bg-blue-50 text-blue-700' : 'hover:bg-slate-50'}`}
                    onClick={() => setShareType('private')}
                  >
                    <UserIcon className="h-4 w-4" />
                    Private
                  </button>
                  <button
                    type="button"
                    className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-xs transition-colors ${shareType === 'public_anyone' ? 'border-blue-500 bg-blue-50 text-blue-700' : 'hover:bg-slate-50'}`}
                    onClick={() => {
                      setShareType('public_anyone');
                      setSelectedUser('');
                      setSelectedGroup('');
                      setSearchQuery('');
                    }}
                  >
                    <Globe className="h-4 w-4" />
                    Public (anyone)
                  </button>
                  <button
                    type="button"
                    className={`flex flex-col items-center gap-1.5 rounded-lg border p-3 text-xs transition-colors ${shareType === 'public_logged_in' ? 'border-blue-500 bg-blue-50 text-blue-700' : 'hover:bg-slate-50'}`}
                    onClick={() => {
                      setShareType('public_logged_in');
                      setSelectedUser('');
                      setSelectedGroup('');
                      setSearchQuery('');
                    }}
                  >
                    <LogIn className="h-4 w-4" />
                    Public (logged in)
                  </button>
                </div>
              </div>

              {shareType === 'private' && (
                <>
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
                </>
              )}

              {shareType !== 'private' && (
                <div className="rounded-lg border border-dashed p-4 text-center text-sm text-muted-foreground">
                  {shareType === 'public_anyone'
                    ? 'Anyone with the link can access this file — no signup or login required.'
                    : 'Any logged-in user with the link can access this file.'}
                </div>
              )}

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
                disabled={(shareType === 'private' && !selectedUser && !selectedGroup) || isLoading}
              >
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {shareType === 'private' ? 'Share' : 'Create Public Link'}
              </Button>
            </div>
          </TabsContent>
          
          <TabsContent value="permissions" className="py-4 overflow-hidden">
            <ScrollArea className="h-[300px] pr-4">
              <div className="space-y-4 overflow-hidden">
                {shares.length === 0 ? (
                  <div className="text-center text-sm text-muted-foreground py-8">
                    No one has access to this item yet.
                  </div>
                ) : (
                  shares.map((share) => (
                    <div key={share.id} className="rounded-lg border overflow-hidden">
                      <div className="flex items-center justify-between space-x-4 p-3">
                        <div className="flex items-center space-x-3">
                          <Avatar className="h-8 w-8">
                            {share.shareType === 'public_anyone' ? (
                              <AvatarFallback className="bg-green-100 text-green-700"><Globe className="h-4 w-4" /></AvatarFallback>
                            ) : share.shareType === 'public_logged_in' ? (
                              <AvatarFallback className="bg-amber-100 text-amber-700"><LogIn className="h-4 w-4" /></AvatarFallback>
                            ) : share.sharedWithUser ? (
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
                              {share.shareType === 'public_anyone'
                                ? 'Public (anyone)'
                                : share.shareType === 'public_logged_in'
                                ? 'Public (logged in)'
                                : share.sharedWithUser 
                                ? `${share.sharedWithUser.firstName} ${share.sharedWithUser.lastName}`
                                : share.sharedWithGroup?.name || 'Unknown Group'}
                            </p>
                            <p className="text-xs text-muted-foreground">
                              {share.shareType === 'public_anyone'
                                ? 'Anyone with the link'
                                : share.shareType === 'public_logged_in'
                                ? 'Any logged-in user'
                                : share.sharedWithUser ? share.sharedWithUser.email : 'Group'}
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
                      {(share.shareType === 'public_anyone' || share.shareType === 'public_logged_in') && (
                        <div className="border-t bg-slate-50/50 px-3 py-2">
                          <CopyLinkButton fileId={share.fileID} />
                        </div>
                      )}
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
