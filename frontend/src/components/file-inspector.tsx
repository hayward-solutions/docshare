'use client';

import { useState, useEffect, useCallback } from 'react';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet';
import { Separator } from '@/components/ui/separator';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Card, CardContent } from '@/components/ui/card';
import { FileIconComponent } from '@/components/file-icon';
import { Loader2, Calendar, HardDrive, User as UserIcon, Users, Globe, LogIn, Check, Link } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { apiMethods } from '@/lib/api';
import { File, Share } from '@/lib/types';
import { format } from 'date-fns';
import { toast } from 'sonner';

function CopyPublicLinkButton({ fileId }: { fileId: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    const url = typeof window !== 'undefined' ? `${window.location.origin}/shared/${fileId}` : '';
    await navigator.clipboard.writeText(url);
    setCopied(true);
    toast.success('Link copied');
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Button variant="outline" size="sm" className="h-6 text-[11px] mt-1.5" onClick={handleCopy}>
      {copied ? <Check className="mr-1 h-3 w-3 text-green-600" /> : <Link className="mr-1 h-3 w-3" />}
      {copied ? 'Copied' : 'Copy link'}
    </Button>
  );
}

interface FileInspectorProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  fileId: string | null;
}

export function FileInspector({ open, onOpenChange, fileId }: FileInspectorProps) {
  const [file, setFile] = useState<File | null>(null);
  const [shares, setShares] = useState<Share[]>([]);
  const [isLoading, setIsLoading] = useState(false);

  const fetchData = useCallback(async () => {
    if (!fileId) return;
    
    setIsLoading(true);
    try {
      const fileRes = await apiMethods.get<File>(`/api/files/${fileId}`);
      if (fileRes.success && fileRes.data) {
        setFile(fileRes.data);
      } else {
        throw new Error('Failed to load file details');
      }

      const sharesRes = await apiMethods.get<Share[]>(`/api/files/${fileId}/shares`);
      if (sharesRes.success && sharesRes.data) {
        setShares(sharesRes.data);
      } else {
        console.error('Failed to load shares:', sharesRes.error);
        toast.error('Failed to load sharing information');
      }
    } catch (error) {
      console.error('Failed to fetch file info:', error);
      toast.error('Failed to load file information');
    } finally {
      setIsLoading(false);
    }
  }, [fileId]);

  useEffect(() => {
    if (open && fileId) {
      fetchData();
    } else if (!open) {
      setFile(null);
      setShares([]);
    }
  }, [open, fileId, fetchData]);

  const formatBytes = (bytes: number, decimals = 2) => {
    if (!+bytes) return '0 Bytes';
    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;
    const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
  };

  const getPermissionBadge = (permission: string) => {
    switch (permission) {
      case 'edit':
        return <Badge variant="default" className="bg-blue-600 hover:bg-blue-700">Editor</Badge>;
      case 'download':
      case 'view':
        return <Badge variant="secondary">Viewer</Badge>;
      default:
        return <Badge variant="outline">{permission}</Badge>;
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-[400px] sm:w-[540px] p-0">
        <div className="flex h-full flex-col overflow-hidden">
          <SheetHeader className="shrink-0 px-6 py-4 border-b">
            <SheetTitle>File Details</SheetTitle>
            <SheetDescription>
              View metadata and access information.
            </SheetDescription>
          </SheetHeader>
          
          <ScrollArea className="min-h-0 flex-1">
            {isLoading ? (
              <div className="flex h-full items-center justify-center py-20">
                <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
              </div>
            ) : file ? (
              <div className="space-y-6 p-6">
                <div className="flex flex-col items-center space-y-4">
                  <div className="rounded-xl border bg-slate-50 p-6 shadow-sm">
                    <FileIconComponent 
                      mimeType={file.mimeType} 
                      isDirectory={file.isDirectory} 
                      className="h-20 w-20 text-blue-600" 
                    />
                  </div>
                  <div className="text-center">
                    <h3 className="text-lg font-semibold break-all">{file.name}</h3>
                    <p className="text-sm text-muted-foreground">{file.isDirectory ? 'Folder' : file.mimeType}</p>
                    {file.sharedWith && file.sharedWith > 0 && (
                      <div className="flex items-center justify-center gap-1 mt-2">
                        <Users className="h-4 w-4 text-muted-foreground" />
                        <span className="text-xs text-muted-foreground">Shared with {file.sharedWith} user{file.sharedWith !== 1 ? 's' : ''}</span>
                      </div>
                    )}
                  </div>
                </div>

                <Separator />

                <div className="grid grid-cols-2 gap-4">
                  <Card>
                    <CardContent className="p-4 flex flex-col items-center justify-center text-center space-y-2">
                      <HardDrive className="h-5 w-5 text-slate-500" />
                      <div>
                        <p className="text-xs text-muted-foreground">Size</p>
                        <p className="font-medium">{file.isDirectory ? '-' : formatBytes(file.size)}</p>
                      </div>
                    </CardContent>
                  </Card>
                  <Card>
                    <CardContent className="p-4 flex flex-col items-center justify-center text-center space-y-2">
                      <Calendar className="h-5 w-5 text-slate-500" />
                      <div>
                        <p className="text-xs text-muted-foreground">Created</p>
                        <p className="font-medium">{format(new Date(file.createdAt), 'MMM d, yyyy')}</p>
                      </div>
                    </CardContent>
                  </Card>
                </div>

                <div className="space-y-1">
                  <p className="text-xs text-muted-foreground">Last Modified</p>
                  <p className="text-sm font-medium">{format(new Date(file.updatedAt), 'PPpp')}</p>
                </div>

                <Separator />

                <div className="space-y-3">
                  <h4 className="text-sm font-medium flex items-center gap-2">
                    <UserIcon className="h-4 w-4" /> Owner
                  </h4>
                  {file.owner ? (
                    <div className="flex items-center space-x-3 rounded-lg border p-3 bg-slate-50/50">
                      <Avatar>
                        <AvatarImage src={file.owner.avatarURL} />
                        <AvatarFallback>{file.owner.firstName[0]}</AvatarFallback>
                      </Avatar>
                      <div className="flex-1 overflow-hidden">
                        <p className="text-sm font-medium truncate">
                          {file.owner.firstName} {file.owner.lastName}
                        </p>
                        <p className="text-xs text-muted-foreground truncate">{file.owner.email}</p>
                      </div>
                      <Badge variant="outline" className="ml-auto border-blue-200 bg-blue-50 text-blue-700">Owner</Badge>
                    </div>
                  ) : (
                    <p className="text-sm text-muted-foreground">Unknown owner</p>
                  )}
                </div>

                <Separator />

                <div className="space-y-3">
                   <div className="flex items-center justify-between">
                    <h4 className="text-sm font-medium flex items-center gap-2">
                      <Users className="h-4 w-4" /> Who has access
                    </h4>
                    <Badge variant="secondary" className="text-xs">
                      {shares.length + (file.owner ? 1 : 0)} users
                    </Badge>
                  </div>
                  
                  <div className="space-y-2">
                    {file.owner && (
                      <div className="flex items-center justify-between space-x-3 rounded-lg p-2 hover:bg-slate-50 transition-colors">
                        <div className="flex items-center space-x-3 overflow-hidden">
                          <Avatar className="h-8 w-8">
                            <AvatarImage src={file.owner.avatarURL} />
                            <AvatarFallback>{file.owner.firstName[0]}</AvatarFallback>
                          </Avatar>
                          <div className="truncate">
                            <p className="text-sm font-medium truncate">
                              {file.owner.firstName} {file.owner.lastName}
                            </p>
                            <p className="text-xs text-muted-foreground truncate">{file.owner.email}</p>
                          </div>
                        </div>
                        <Badge variant="outline" className="border-blue-200 bg-blue-50 text-blue-700 shrink-0">Owner</Badge>
                      </div>
                    )}

                    {shares.map((share) => (
                      <div key={share.id} className="rounded-lg p-2 hover:bg-slate-50 transition-colors">
                        <div className="flex items-center justify-between space-x-3">
                          <div className="flex items-center space-x-3 overflow-hidden">
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
                            <div className="truncate">
                              <p className="text-sm font-medium truncate">
                                {share.shareType === 'public_anyone'
                                  ? 'Public (anyone)'
                                  : share.shareType === 'public_logged_in'
                                  ? 'Public (logged in)'
                                  : share.sharedWithUser 
                                  ? `${share.sharedWithUser.firstName} ${share.sharedWithUser.lastName}`
                                  : share.sharedWithGroup?.name || 'Unknown Group'}
                              </p>
                              <p className="text-xs text-muted-foreground truncate">
                                {share.shareType === 'public_anyone'
                                  ? 'Anyone with the link'
                                  : share.shareType === 'public_logged_in'
                                  ? 'Any logged-in user'
                                  : share.sharedWithUser ? share.sharedWithUser.email : 'Group'}
                              </p>
                            </div>
                          </div>
                          <div className="shrink-0">
                            {getPermissionBadge(share.permission)}
                          </div>
                        </div>
                        {(share.shareType === 'public_anyone' || share.shareType === 'public_logged_in') && (
                          <div className="ml-11">
                            <CopyPublicLinkButton fileId={share.fileID} />
                          </div>
                        )}
                      </div>
                    ))}
                    
                    {shares.length === 0 && !file.owner && (
                      <p className="text-sm text-muted-foreground py-2">No sharing information available.</p>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex h-full flex-col items-center justify-center p-6 text-center text-muted-foreground">
                <FileIconComponent mimeType="" isDirectory={false} className="h-12 w-12 mb-4 opacity-20" />
                <p>Select a file to view details</p>
              </div>
            )}
          </ScrollArea>
        </div>
      </SheetContent>
    </Sheet>
  );
}
