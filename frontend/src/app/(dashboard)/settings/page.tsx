'use client';

import { useState, useEffect } from 'react';
import { useAuth } from '@/lib/auth';
import { userAPI, auditAPI, tokenAPI } from '@/lib/api';
import { Group, GroupMembership, APIToken } from '@/lib/types';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';
import { User as UserIcon, Lock, Users, Upload, Shield, FileText, Download, Key, Plus, Copy, Trash2, AlertTriangle } from 'lucide-react';
import { formatDistanceToNow } from 'date-fns';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export default function AccountSettingsPage() {
  const { user, loadUser } = useAuth();
  const [isLoading, setIsLoading] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [isLoadingGroups, setIsLoadingGroups] = useState(false);

  // Profile form state
  const [firstName, setFirstName] = useState('');
  const [lastName, setLastName] = useState('');
  const [avatarUrl, setAvatarUrl] = useState('');

  // Password form state
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [isLoadingTokens, setIsLoadingTokens] = useState(false);
  const [isCreateTokenOpen, setIsCreateTokenOpen] = useState(false);
  const [newTokenName, setNewTokenName] = useState('');
  const [newTokenExpiry, setNewTokenExpiry] = useState('30d');
  const [createdToken, setCreatedToken] = useState<string | null>(null);

  useEffect(() => {
    const fetchGroups = async () => {
      setIsLoadingGroups(true);
      try {
        const res = await userAPI.getGroups();
        if (res.success && res.data) {
          setGroups(res.data);
        }
      } catch (error) {
        console.error('Failed to fetch groups:', error);
      } finally {
        setIsLoadingGroups(false);
      }
    };

    const fetchTokens = async () => {
      setIsLoadingTokens(true);
      try {
        const res = await tokenAPI.list();
        if (res.success && res.data) {
          setTokens(res.data);
        }
      } catch (error) {
        console.error('Failed to fetch tokens:', error);
      } finally {
        setIsLoadingTokens(false);
      }
    };

    if (user) {
      setFirstName(user.firstName);
      setLastName(user.lastName);
      setAvatarUrl(user.avatarURL || '');
      fetchGroups();
      fetchTokens();
    }
  }, [user]);

  const handleProfileUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);

    try {
      const res = await userAPI.updateProfile({
        firstName: firstName.trim(),
        lastName: lastName.trim(),
        avatarURL: avatarUrl.trim() || undefined,
      });

      if (res.success) {
        toast.success('Profile updated successfully');
        await loadUser();
      }
    } catch (error) {
      toast.error('Failed to update profile');
      console.error('Profile update error:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handlePasswordChange = async (e: React.FormEvent) => {
    e.preventDefault();

    if (newPassword.length < 8) {
      toast.error('New password must be at least 8 characters');
      return;
    }

    if (newPassword !== confirmPassword) {
      toast.error('New passwords do not match');
      return;
    }

    setIsLoading(true);

    try {
      const res = await userAPI.changePassword(currentPassword, newPassword);

      if (res.success) {
        toast.success('Password changed successfully');
        setCurrentPassword('');
        setNewPassword('');
        setConfirmPassword('');
      }
    } catch (error) {
      toast.error('Failed to change password');
      console.error('Password change error:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleAvatarUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (file.size > 5 * 1024 * 1024) {
      toast.error('Avatar must be smaller than 5MB');
      return;
    }

    if (!file.type.startsWith('image/')) {
      toast.error('Avatar must be an image');
      return;
    }

    setIsLoading(true);
    try {
      const res = await userAPI.uploadAvatar(file);
      if (res.success && res.data && res.data.avatarURL) {
        setAvatarUrl(res.data.avatarURL);
        toast.success('Avatar uploaded successfully');
      } else {
        toast.error('Failed to get upload URL');
      }
    } catch (error) {
      toast.error('Failed to upload avatar');
      console.error('Avatar upload error:', error);
    } finally {
      setIsLoading(false);
    }
  };

  const handleDownloadAuditLog = async (format: 'csv' | 'json') => {
    try {
      const blob = await auditAPI.download(format);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `audit-log.${format}`;
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
      toast.success(`Audit log downloaded as ${format.toUpperCase()}`);
    } catch (error) {
      console.error('Download error:', error);
      toast.error('Failed to download audit log');
    }
  };

  const getRoleBadgeVariant = (role: string) => {
    switch (role) {
      case 'owner':
        return 'default';
      case 'admin':
        return 'secondary';
      case 'member':
        return 'outline';
      default:
        return 'outline';
    }
  };

  const handleCreateToken = async () => {
    if (!newTokenName.trim()) {
      toast.error('Token name is required');
      return;
    }

    setIsLoadingTokens(true);
    try {
      const res = await tokenAPI.create({
        name: newTokenName,
        expiresIn: newTokenExpiry,
      });

      if (res.success && res.data) {
        setCreatedToken(res.data.token);
        setTokens([res.data.apiToken, ...tokens]);
        setNewTokenName('');
        setNewTokenExpiry('30d');
        setIsCreateTokenOpen(false);
        toast.success('API token created successfully');
      }
    } catch (error) {
      toast.error('Failed to create API token');
      console.error('Create token error:', error);
    } finally {
      setIsLoadingTokens(false);
    }
  };

  const handleRevokeToken = async (id: string) => {
    try {
      const res = await tokenAPI.revoke(id);
      if (res.success) {
        setTokens(tokens.filter((t) => t.id !== id));
        toast.success('API token revoked successfully');
      }
    } catch (error) {
      toast.error('Failed to revoke API token');
      console.error('Revoke token error:', error);
    }
  };

  const handleCopyToken = () => {
    if (createdToken) {
      navigator.clipboard.writeText(createdToken);
      toast.success('Token copied to clipboard');
    }
  };

  if (!user) {
    return <div>Loading...</div>;
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      <div className="space-y-2">
        <h1 className="text-3xl font-bold">Account Settings</h1>
        <p className="text-muted-foreground">
          Manage your profile, password, and group memberships
        </p>
      </div>

      <Tabs defaultValue="profile" className="space-y-6">
        <TabsList className="grid w-full grid-cols-5">
          <TabsTrigger value="profile" className="flex items-center gap-2">
            <UserIcon className="h-4 w-4" />
            Profile
          </TabsTrigger>
          <TabsTrigger value="security" className="flex items-center gap-2">
            <Lock className="h-4 w-4" />
            Security
          </TabsTrigger>
          <TabsTrigger value="groups" className="flex items-center gap-2">
            <Users className="h-4 w-4" />
            Groups
          </TabsTrigger>
          <TabsTrigger value="api-tokens" className="flex items-center gap-2">
            <Key className="h-4 w-4" />
            API Tokens
          </TabsTrigger>
          <TabsTrigger value="audit" className="flex items-center gap-2">
            <FileText className="h-4 w-4" />
            Audit Log
          </TabsTrigger>
        </TabsList>

        <TabsContent value="profile">
          <Card>
            <CardHeader>
              <CardTitle>Profile Information</CardTitle>
              <CardDescription>
                Update your personal information and avatar
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <form onSubmit={handleProfileUpdate} className="space-y-4">
                <div className="flex items-center gap-4">
                  <Avatar className="h-20 w-20">
                    <AvatarImage src={avatarUrl} />
                    <AvatarFallback className="text-lg">
                      {firstName[0]}{lastName[0]}
                    </AvatarFallback>
                  </Avatar>
                  <div className="space-y-2">
                    <Label htmlFor="avatar-upload" className="cursor-pointer">
                      <div className="flex items-center gap-2">
                        <Upload className="h-4 w-4" />
                        Change Avatar
                      </div>
                      <Input
                        id="avatar-upload"
                        type="file"
                        accept="image/*"
                        onChange={handleAvatarUpload}
                        className="hidden"
                      />
                    </Label>
                    <p className="text-sm text-muted-foreground">
                      JPG, PNG or GIF. Max 5MB.
                    </p>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                  <div className="space-y-2">
                    <Label htmlFor="firstName">First Name</Label>
                    <Input
                      id="firstName"
                      value={firstName}
                      onChange={(e) => setFirstName(e.target.value)}
                      required
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="lastName">Last Name</Label>
                    <Input
                      id="lastName"
                      value={lastName}
                      onChange={(e) => setLastName(e.target.value)}
                      required
                    />
                  </div>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    value={user.email}
                    disabled
                    className="bg-muted"
                  />
                  <p className="text-sm text-muted-foreground">
                    Email cannot be changed
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="avatarUrl">Avatar URL</Label>
                  <Input
                    id="avatarUrl"
                    type="url"
                    value={avatarUrl}
                    onChange={(e) => setAvatarUrl(e.target.value)}
                    placeholder="https://example.com/avatar.jpg"
                  />
                </div>

                <Button type="submit" disabled={isLoading}>
                  {isLoading ? 'Saving...' : 'Save Changes'}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="security">
          <Card>
            <CardHeader>
              <CardTitle>Change Password</CardTitle>
              <CardDescription>
                Update your password to keep your account secure
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={handlePasswordChange} className="space-y-4">
                <div className="space-y-2">
                  <Label htmlFor="currentPassword">Current Password</Label>
                  <Input
                    id="currentPassword"
                    type="password"
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    required
                  />
                </div>

                <div className="space-y-2">
                  <Label htmlFor="newPassword">New Password</Label>
                  <Input
                    id="newPassword"
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    required
                    minLength={8}
                  />
                  <p className="text-sm text-muted-foreground">
                    Must be at least 8 characters
                  </p>
                </div>

                <div className="space-y-2">
                  <Label htmlFor="confirmPassword">Confirm New Password</Label>
                  <Input
                    id="confirmPassword"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    required
                    minLength={8}
                  />
                </div>

                <Button type="submit" disabled={isLoading}>
                  {isLoading ? 'Changing...' : 'Change Password'}
                </Button>
              </form>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="groups">
          <Card>
            <CardHeader>
              <CardTitle>Group Memberships</CardTitle>
              <CardDescription>
                View your groups and roles within them
              </CardDescription>
            </CardHeader>
            <CardContent>
              {isLoadingGroups ? (
                <div className="text-center py-8">Loading groups...</div>
              ) : groups.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  You are not a member of any groups
                </div>
              ) : (
                <div className="space-y-4">
                  {groups.map((group) => {
                    const membership = group.memberships?.find(
                      (m: GroupMembership) => m.userID === user.id
                    );

                    if (!membership) return null;

                    return (
                      <div
                        key={group.id}
                        className="flex items-center justify-between p-4 border rounded-lg"
                      >
                        <div className="space-y-1">
                          <h3 className="font-medium">{group.name}</h3>
                          {group.description && (
                            <p className="text-sm text-muted-foreground">
                              {group.description}
                            </p>
                          )}
                        </div>
                        <div className="flex items-center gap-2">
                          <Badge variant={getRoleBadgeVariant(membership.role)}>
                            {membership.role}
                          </Badge>
                          {membership.role === 'owner' && (
                            <Shield className="h-4 w-4 text-muted-foreground" />
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="api-tokens">
          <Card>
            <CardHeader>
              <CardTitle>API Tokens</CardTitle>
              <CardDescription>
                Manage your personal access tokens for CLI and API access
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              {createdToken && (
                <Alert className="bg-green-50 border-green-200 text-green-900 [&>svg]:text-green-600">
                  <AlertTriangle className="h-4 w-4" />
                  <AlertTitle className="text-green-800">Token Created Successfully</AlertTitle>
                  <AlertDescription className="text-green-700">
                    <p className="mb-2">Make sure to copy your personal access token now. You won't be able to see it again!</p>
                    <div className="flex items-center gap-2 mt-2">
                      <code className="bg-white px-2 py-1 rounded border border-green-200 font-mono text-sm flex-1 break-all">
                        {createdToken}
                      </code>
                      <Button size="sm" variant="outline" onClick={handleCopyToken} className="h-8 shrink-0 bg-white hover:bg-green-50 border-green-200 text-green-700">
                        <Copy className="h-4 w-4 mr-2" />
                        Copy
                      </Button>
                    </div>
                  </AlertDescription>
                </Alert>
              )}

              <div className="flex justify-end">
                <Dialog open={isCreateTokenOpen} onOpenChange={setIsCreateTokenOpen}>
                  <DialogTrigger asChild>
                    <Button>
                      <Plus className="h-4 w-4 mr-2" />
                      Create Token
                    </Button>
                  </DialogTrigger>
                  <DialogContent>
                    <DialogHeader>
                      <DialogTitle>Create API Token</DialogTitle>
                      <DialogDescription>
                        Generate a new personal access token.
                      </DialogDescription>
                    </DialogHeader>
                    <div className="space-y-4 py-4">
                      <div className="space-y-2">
                        <Label htmlFor="token-name">Name</Label>
                        <Input
                          id="token-name"
                          placeholder="e.g. CLI Access"
                          value={newTokenName}
                          onChange={(e) => setNewTokenName(e.target.value)}
                        />
                      </div>
                      <div className="space-y-2">
                        <Label htmlFor="token-expiry">Expiration</Label>
                        <Select value={newTokenExpiry} onValueChange={setNewTokenExpiry}>
                          <SelectTrigger>
                            <SelectValue placeholder="Select expiration" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="30d">30 days</SelectItem>
                            <SelectItem value="90d">90 days</SelectItem>
                            <SelectItem value="365d">1 year</SelectItem>
                            <SelectItem value="never">Never</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>
                    <DialogFooter>
                      <Button variant="outline" onClick={() => setIsCreateTokenOpen(false)}>Cancel</Button>
                      <Button onClick={handleCreateToken} disabled={isLoadingTokens}>
                        {isLoadingTokens ? 'Creating...' : 'Create'}
                      </Button>
                    </DialogFooter>
                  </DialogContent>
                </Dialog>
              </div>

              {isLoadingTokens && tokens.length === 0 ? (
                <div className="text-center py-8">Loading tokens...</div>
              ) : tokens.length === 0 ? (
                <div className="text-center py-8 text-muted-foreground">
                  You don't have any API tokens yet.
                </div>
              ) : (
                <div className="border rounded-md">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Name</TableHead>
                        <TableHead>Prefix</TableHead>
                        <TableHead>Created</TableHead>
                        <TableHead>Last Used</TableHead>
                        <TableHead>Expires</TableHead>
                        <TableHead className="text-right">Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {tokens.map((token) => (
                        <TableRow key={token.id}>
                          <TableCell className="font-medium">{token.name}</TableCell>
                          <TableCell className="font-mono text-xs text-muted-foreground">
                            {token.prefix}****
                          </TableCell>
                          <TableCell>
                            {formatDistanceToNow(new Date(token.createdAt), { addSuffix: true })}
                          </TableCell>
                          <TableCell>
                            {token.lastUsedAt
                              ? formatDistanceToNow(new Date(token.lastUsedAt), { addSuffix: true })
                              : 'Never used'}
                          </TableCell>
                          <TableCell>
                            {token.expiresAt
                              ? formatDistanceToNow(new Date(token.expiresAt), { addSuffix: true })
                              : 'Never'}
                          </TableCell>
                          <TableCell className="text-right">
                            <Button
                              variant="ghost"
                              size="sm"
                              className="text-destructive hover:text-destructive hover:bg-destructive/10"
                              onClick={() => handleRevokeToken(token.id)}
                            >
                              <Trash2 className="h-4 w-4" />
                              <span className="sr-only">Revoke</span>
                            </Button>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="audit">
          <Card>
            <CardHeader>
              <CardTitle>Audit Log</CardTitle>
              <CardDescription>
                Download a log of your account activity
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="flex gap-4">
                <Button onClick={() => handleDownloadAuditLog('csv')} className="flex items-center gap-2">
                  <Download className="h-4 w-4" />
                  Download CSV
                </Button>
                <Button onClick={() => handleDownloadAuditLog('json')} variant="outline" className="flex items-center gap-2">
                  <Download className="h-4 w-4" />
                  Download JSON
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}