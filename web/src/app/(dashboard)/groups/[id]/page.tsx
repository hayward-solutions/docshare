'use client';

import { useState, useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { Group, GroupMembership, User } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { useAuth } from '@/lib/auth';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ArrowLeft, UserPlus, Trash2, Loader2 } from 'lucide-react';
import { Loading } from '@/components/loading';
import { toast } from 'sonner';

export default function GroupDetailPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const { user: currentUser } = useAuth();
  const [group, setGroup] = useState<Group | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [addOpen, setAddOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<User[]>([]);
  const [selectedUserId, setSelectedUserId] = useState('');
  const [memberRole, setMemberRole] = useState('member');
  const [isAdding, setIsAdding] = useState(false);

  const fetchGroup = useCallback(async () => {
    try {
      const res = await apiMethods.get<Group>(`/groups/${id}`);
      if (res.success) {
        setGroup(res.data);
      }
    } catch {
      toast.error('Failed to load group');
      router.push('/groups');
    } finally {
      setIsLoading(false);
    }
  }, [id, router]);

  useEffect(() => {
    fetchGroup();
  }, [fetchGroup]);

  useEffect(() => {
    if (searchQuery.length < 2) {
      setSearchResults([]);
      return;
    }
    const timer = setTimeout(async () => {
      try {
        const res = await apiMethods.get<User[]>('/users/search', { search: searchQuery, limit: 5 });
        const data = res as { success: boolean; data: User[] | { users: User[] } };
        if (data.success) {
          if (Array.isArray(data.data)) {
            setSearchResults(data.data);
          } else if (data.data.users) {
            setSearchResults(data.data.users);
          }
        }
      } catch {
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery]);

  const handleAddMember = async () => {
    if (!selectedUserId) return;
    setIsAdding(true);
    try {
      await apiMethods.post(`/groups/${id}/members`, { userID: selectedUserId, role: memberRole });
      toast.success('Member added');
      setAddOpen(false);
      setSearchQuery('');
      setSelectedUserId('');
      fetchGroup();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Failed to add member';
      toast.error(message);
    } finally {
      setIsAdding(false);
    }
  };

  const handleRemoveMember = async (userId: string) => {
    if (!confirm('Remove this member from the group?')) return;
    try {
      await apiMethods.delete(`/groups/${id}/members/${userId}`);
      toast.success('Member removed');
      fetchGroup();
    } catch {
      toast.error('Failed to remove member');
    }
  };

  const handleDeleteGroup = async () => {
    if (!confirm('Delete this group? This cannot be undone.')) return;
    try {
      await apiMethods.delete(`/groups/${id}`);
      toast.success('Group deleted');
      router.push('/groups');
    } catch {
      toast.error('Failed to delete group');
    }
  };

  const currentMembership = group?.memberships?.find(
    (m: GroupMembership) => m.userID === currentUser?.id
  );
  const isOwnerOrAdmin = currentMembership?.role === 'owner' || currentMembership?.role === 'admin';

  if (isLoading) return <Loading />;
  if (!group) return null;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={() => router.push('/groups')}>
          <ArrowLeft className="h-5 w-5" />
        </Button>
        <div className="flex-1">
          <h1 className="text-2xl font-bold">{group.name}</h1>
          {group.description && <p className="text-sm text-muted-foreground">{group.description}</p>}
        </div>
        {isOwnerOrAdmin && (
          <div className="flex gap-2">
            <Dialog open={addOpen} onOpenChange={setAddOpen}>
              <DialogTrigger asChild>
                <Button>
                  <UserPlus className="mr-2 h-4 w-4" />
                  Add Member
                </Button>
              </DialogTrigger>
              <DialogContent className="sm:max-w-[425px]">
                <DialogHeader>
                  <DialogTitle>Add Member</DialogTitle>
                  <DialogDescription>Search for a user to add to this group.</DialogDescription>
                </DialogHeader>
                <div className="grid gap-4 py-4">
                  <div className="space-y-2">
                    <Label>Search User</Label>
                    <Input
                      placeholder="Search by name or email..."
                      value={searchQuery}
                      onChange={(e) => {
                        setSearchQuery(e.target.value);
                        setSelectedUserId('');
                      }}
                    />
                    {searchResults.length > 0 && searchQuery && !selectedUserId && (
                      <div className="rounded-md border bg-popover p-1 shadow-md">
                        {searchResults.map((u) => (
                          <button
                            key={u.id}
                            type="button"
                            className="flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm hover:bg-accent"
                            onClick={() => {
                              setSelectedUserId(u.id);
                              setSearchQuery(`${u.firstName} ${u.lastName}`);
                            }}
                          >
                            <Avatar className="h-6 w-6">
                              <AvatarFallback>{u.firstName[0]}</AvatarFallback>
                            </Avatar>
                            <span>{u.firstName} {u.lastName}</span>
                            <span className="ml-auto text-xs text-muted-foreground">{u.email}</span>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="space-y-2">
                    <Label>Role</Label>
                    <Select value={memberRole} onValueChange={setMemberRole}>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="member">Member</SelectItem>
                        <SelectItem value="admin">Admin</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                </div>
                <DialogFooter>
                  <Button onClick={handleAddMember} disabled={!selectedUserId || isAdding}>
                    {isAdding && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                    Add
                  </Button>
                </DialogFooter>
              </DialogContent>
            </Dialog>
            {currentMembership?.role === 'owner' && (
              <Button variant="destructive" size="sm" onClick={handleDeleteGroup}>
                <Trash2 className="mr-2 h-4 w-4" />
                Delete Group
              </Button>
            )}
          </div>
        )}
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Members ({group.memberships?.length || 0})</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {(group.memberships || []).map((membership: GroupMembership) => (
              <div key={membership.id} className="flex items-center justify-between rounded-lg border p-3">
                <div className="flex items-center gap-3">
                  <Avatar className="h-9 w-9">
                    <AvatarFallback>
                      {membership.user?.firstName?.[0] || '?'}
                      {membership.user?.lastName?.[0] || ''}
                    </AvatarFallback>
                  </Avatar>
                  <div>
                    <p className="text-sm font-medium">
                      {membership.user?.firstName} {membership.user?.lastName}
                    </p>
                    <p className="text-xs text-muted-foreground">{membership.user?.email}</p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Badge variant={membership.role === 'owner' ? 'default' : 'secondary'}>
                    {membership.role}
                  </Badge>
                  {isOwnerOrAdmin && membership.role !== 'owner' && membership.userID !== currentUser?.id && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-8 w-8 text-red-500 dark:text-red-400 hover:text-red-600 dark:hover:text-red-400"
                      onClick={() => handleRemoveMember(membership.userID)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
