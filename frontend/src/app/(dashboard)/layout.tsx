'use client';

import { useState, useEffect } from 'react';
import Link from 'next/link';
import { usePathname, useRouter } from 'next/navigation';
import { useAuth } from '@/lib/auth';
import { activityAPI } from '@/lib/api';
import { 
  Files, 
  Users, 
  Share2, 
  Settings, 
  LogOut, 
  Menu, 
  Shield,
  Bell
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Sheet, SheetContent, SheetTrigger } from '@/components/ui/sheet';
import { cn } from '@/lib/utils';
import { LoadingPage } from '@/components/loading';

import { ScrollArea } from '@/components/ui/scroll-area';

const NavContent = ({ user, pathname, setIsMobileOpen, logout }: {
  user: {
    id?: string;
    firstName?: string;
    lastName?: string;
    email?: string;
    avatarURL?: string;
    role?: string;
  } | null;
  pathname: string;
  setIsMobileOpen: (open: boolean) => void;
  logout: () => void;
}) => {
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    const fetchUnreadCount = async () => {
      try {
        const res = await activityAPI.unreadCount();
        if (res.success && res.data) {
          setUnreadCount(res.data.count);
        }
      } catch (error) {
        console.error('Failed to fetch unread count:', error);
      }
    };
    fetchUnreadCount();
  }, []);

  const navigation = [
    { name: 'My Files', href: '/files', icon: Files },
    { name: 'Shared With Me', href: '/shared', icon: Share2 },
    { name: 'Activity', href: '/activity', icon: Bell, badge: unreadCount > 0 ? unreadCount : undefined },
    { name: 'Groups', href: '/groups', icon: Users },
    { name: 'Account Settings', href: '/settings', icon: Settings },
  ];

  if (user?.role === 'admin') {
    navigation.push({ name: 'Admin', href: '/admin', icon: Shield });
  }

  return (
    <div className="flex h-full flex-col gap-4 py-4">
      <div className="px-6 py-2">
        <h1 className="text-xl font-bold text-white">DocShare</h1>
      </div>
      <ScrollArea className="flex-1 overflow-hidden">
        <nav className="space-y-1 px-3 pr-4">
          {navigation.map((item) => {
            const isActive = pathname.startsWith(item.href);
            return (
              <Link
                key={item.name}
                href={item.href}
                onClick={() => setIsMobileOpen(false)}
                className={cn(
                  'flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-blue-600 text-white'
                    : 'text-slate-300 hover:bg-slate-800 hover:text-white'
                )}
              >
                <item.icon className="h-5 w-5" />
                <span className="flex-1">{item.name}</span>
                {item.badge && (
                  <span className="bg-blue-500 text-white text-xs rounded-full px-1.5 py-0.5 min-w-[1.25rem] text-center">
                    {item.badge}
                  </span>
                )}
              </Link>
            );
          })}
        </nav>
      </ScrollArea>
      <div className="px-3 py-2">
       <div className="flex items-center gap-3 rounded-lg bg-slate-800 px-3 py-3">
         <Avatar className="h-9 w-9">
           <AvatarImage src={user?.avatarURL} />
           <AvatarFallback>{user?.firstName?.[0]}{user?.lastName?.[0]}</AvatarFallback>
         </Avatar>
         <div className="flex flex-1 flex-col overflow-hidden">
           <span className="truncate text-sm font-medium text-white">
             {user?.firstName} {user?.lastName}
           </span>
           <span className="truncate text-xs text-slate-400">{user?.email}</span>
         </div>
         <DropdownMenu>
           <DropdownMenuTrigger asChild>
             <Button variant="ghost" size="icon" className="h-8 w-8 text-slate-400 hover:text-white">
               <Settings className="h-4 w-4" />
             </Button>
           </DropdownMenuTrigger>
           <DropdownMenuContent align="end" className="w-56">
             <DropdownMenuLabel>My Account</DropdownMenuLabel>
             <DropdownMenuSeparator />
             <DropdownMenuItem asChild>
               <Link href="/settings" className="flex items-center">
                 <Settings className="mr-2 h-4 w-4" />
                 Account Settings
               </Link>
             </DropdownMenuItem>
             <DropdownMenuSeparator />
             <DropdownMenuItem onClick={logout} className="text-red-600">
               <LogOut className="mr-2 h-4 w-4" />
               Log out
             </DropdownMenuItem>
           </DropdownMenuContent>
         </DropdownMenu>
       </div>
     </div>
   </div>
  );
};

export default function DashboardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { user, logout, isAuthenticated, isLoading } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const [isMobileOpen, setIsMobileOpen] = useState(false);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    return <LoadingPage />;
  }

  return (
    <div className="flex h-screen overflow-hidden bg-slate-50">
      <div className="hidden w-64 shrink-0 flex-col bg-slate-900 md:flex">
        <NavContent user={user} pathname={pathname} setIsMobileOpen={setIsMobileOpen} logout={logout} />
      </div>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-16 shrink-0 items-center justify-between border-b bg-white px-4 md:hidden">
          <h1 className="text-lg font-bold">DocShare</h1>
          <Sheet open={isMobileOpen} onOpenChange={setIsMobileOpen}>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon">
                <Menu className="h-6 w-6" />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-64 bg-slate-900 p-0 border-r-slate-800">
              <NavContent user={user} pathname={pathname} setIsMobileOpen={setIsMobileOpen} logout={logout} />
            </SheetContent>
          </Sheet>
        </header>

        <main className="flex-1 overflow-y-auto p-4 md:p-8">
          {children}
        </main>
      </div>
    </div>
  );
}
