import type { ReactNode } from 'react';
import { useApp } from '../context/AppContext';
import { hasRole } from '../utils/format';
import type { Role } from '../api/types';

interface RoleGateProps {
  required: Role;
  children: ReactNode;
  fallback?: ReactNode;
  reason?: string;
}

export function RoleGate({ required, children, fallback, reason }: RoleGateProps) {
  const { role } = useApp();
  if (hasRole(role, required)) return <>{children}</>;
  return (
    <>
      {fallback ?? (
        <span className="role-blocked" title={reason}>
          Insufficient role (needs {required})
        </span>
      )}
    </>
  );
}
