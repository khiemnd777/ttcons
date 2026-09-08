import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import clsx from 'clsx';
import { downloadBlob } from '../lib/api/client';
import type { User } from '../lib/types/domain';

type UserAvatarSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';

const sizeClass: Record<UserAvatarSize, string> = {
  xs: 'h-7 w-7 text-[10px]',
  sm: 'h-8 w-8 text-xs',
  md: 'h-10 w-10 text-sm',
  lg: 'h-14 w-14 text-lg',
  xl: 'h-16 w-16 text-xl',
};

export function UserAvatar({ user, size = 'md', className, decorative = false }: { user: User; size?: UserAvatarSize; className?: string; decorative?: boolean }) {
  const avatar = useQuery({
    queryKey: ['own-avatar', user.id, user.avatar_updated_at ?? ''],
    queryFn: () => downloadBlob('/auth/avatar'),
    enabled: Boolean(user.has_avatar),
    retry: false,
    staleTime: Infinity,
  });
  const source = useObjectURL(avatar.data);
  const label = `Ảnh đại diện của ${user.full_name}`;
  const initial = userInitials(user.full_name);

  if (source) {
    return <img className={clsx('shrink-0 rounded-full object-cover', sizeClass[size], className)} src={source} alt={decorative ? '' : label} aria-hidden={decorative || undefined} />;
  }
  return <span className={clsx('grid shrink-0 place-items-center rounded-full bg-blue-100 font-bold text-blue-700', sizeClass[size], className)} aria-label={decorative ? undefined : label} aria-hidden={decorative || undefined}>{initial}</span>;
}

function useObjectURL(blob: Blob | undefined) {
  const [source, setSource] = useState('');
  useEffect(() => {
    if (!blob) {
      setSource('');
      return undefined;
    }
    const next = URL.createObjectURL(blob);
    setSource(next);
    return () => URL.revokeObjectURL(next);
  }, [blob]);
  return source;
}

export function userInitials(fullName: string) {
  return fullName
    .split(/\s+/)
    .filter(Boolean)
    .slice(-2)
    .map((part) => part[0]?.toUpperCase())
    .join('') || 'U';
}
