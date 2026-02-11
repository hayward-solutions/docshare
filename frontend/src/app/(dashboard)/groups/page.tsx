'use client';

import { useState, useEffect, useCallback } from 'react';
import Link from 'next/link';
import { Group } from '@/lib/types';
import { apiMethods } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Users, Plus, Loader2 } from 'lucide-react';
import { Loading } from '@/components/loading';
import { toast } from 'sonner';

export default function GroupsPage() {
  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [isCreating, setIsCreating] = useState(false);

  const fetchGroups = useCallback(async () => {
    try {
      const res = await apiMethods.get<Group[]>('/api/groups');
      if (res.success) {
        setGroups(res.data);
      }
    } catch {
      toast.error('Failed to load groups');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchGroups();
  }, [fetchGroups]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsCreating(true);
    try {
      await apiMethods.post('/api/groups', { name, description: description || undefined });
      toast.success('Group created');
      setCreateOpen(false);
      setName('');
      setDescription('');
      fetchGroups();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : 'Failed to create group';
      toast.error(message);
    } finally {
      setIsCreating(false);
    }
  };

  if (isLoading) return <Loading />;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Groups</h1>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-2 h-4 w-4" />
              New Group
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-[425px]">
            <DialogHeader>
              <DialogTitle>Create Group</DialogTitle>
              <DialogDescription>Create a new group to share files with multiple users.</DialogDescription>
            </DialogHeader>
            <form onSubmit={handleCreate}>
              <div className="grid gap-4 py-4">
                <div className="space-y-2">
                  <Label htmlFor="group-name">Name</Label>
                  <Input id="group-name" value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="group-desc">Description</Label>
                  <Input id="group-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
                </div>
              </div>
              <DialogFooter>
                <Button type="submit" disabled={isCreating || !name.trim()}>
                  {isCreating && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      {groups.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-12 text-center">
          <Users className="h-12 w-12 text-slate-300" />
          <h3 className="mt-4 text-lg font-semibold">No groups yet</h3>
          <p className="text-sm text-muted-foreground">Create a group to start sharing files with your team.</p>
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {groups.map((group) => (
            <Link key={group.id} href={`/groups/${group.id}`}>
              <Card className="transition-shadow hover:shadow-md cursor-pointer">
                <CardHeader className="pb-3">
                  <CardTitle className="text-lg">{group.name}</CardTitle>
                </CardHeader>
                <CardContent>
                  <p className="text-sm text-muted-foreground line-clamp-2">
                    {group.description || 'No description'}
                  </p>
                  <div className="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
                    <Users className="h-3.5 w-3.5" />
                    <span>{group.memberships?.length || 0} members</span>
                  </div>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
