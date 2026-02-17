export interface User {
  id: string;
  email: string;
  firstName: string;
  lastName: string;
  avatarURL?: string;
  role: 'user' | 'admin';
  createdAt: string;
  authProvider?: string;
}

export interface Group {
  id: string;
  name: string;
  description?: string;
  createdByID: string;
  createdAt: string;
  memberships?: GroupMembership[];
  memberCount?: number;
}

export interface GroupMembership {
  id: string;
  groupID: string;
  userID: string;
  role: 'owner' | 'admin' | 'member';
  user?: User;
  createdAt: string;
}

export interface File {
  id: string;
  name: string;
  mimeType: string;
  size: number;
  isDirectory: boolean;
  parentID?: string;
  ownerID: string;
  storagePath: string;
  createdAt: string;
  updatedAt: string;
  owner?: User;
  shared?: boolean;
  sharedWith?: number;
  parentName?: string;
}

export type ShareType = 'private' | 'public_anyone' | 'public_logged_in';

export interface Share {
  id: string;
  fileID: string;
  sharedByID: string;
  sharedWithUserID?: string;
  sharedWithGroupID?: string;
  shareType: ShareType;
  permission: 'view' | 'download' | 'edit';
  expiresAt?: string;
  createdAt: string;
  file?: File;
  sharedBy?: User;
  sharedWithUser?: User;
  sharedWithGroup?: Group;
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  totalPages: number;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T;
  error?: string;
  pagination?: Pagination;
}

export interface LoginResponse {
  token: string;
  user: User;
}

export interface MFALoginResponse {
  mfaRequired: true;
  mfaToken: string;
  methods: ('totp' | 'webauthn')[];
}

export interface MFAStatus {
  mfaEnabled: boolean;
  totpEnabled: boolean;
  totpVerifiedAt: string | null;
  webauthnEnabled: boolean;
  webauthnCredentialsCount: number;
  recoveryCodesRemaining: number;
}

export interface TOTPSetupResponse {
  secret: string;
  qrUri: string;
}

export interface RecoveryCodesResponse {
  recoveryCodes: string[];
}

export interface WebAuthnCredentialInfo {
  id: string;
  name: string;
  lastUsedAt: string | null;
  createdAt: string;
  backupEligible: boolean;
  backupState: boolean;
}

export interface PasskeyRegisterResponse {
  credential: WebAuthnCredentialInfo;
  recoveryCodes?: string[];
}

export interface BreadcrumbItem {
  id: string;
  name: string;
}

export interface Activity {
  id: string;
  userID: string;
  actorID: string;
  action: string;
  resourceType: string;
  resourceID?: string;
  resourceName: string;
  message: string;
  isRead: boolean;
  createdAt: string;
  actor?: User;
}

export interface AuditLogEntry {
  id: string;
  userID?: string;
  action: string;
  resourceType: string;
  resourceID?: string;
  details?: Record<string, unknown>;
  ipAddress: string;
  requestID?: string;
  createdAt: string;
}

export interface APIToken {
  id: string;
  userID: string;
  name: string;
  prefix: string;
  expiresAt?: string;
  lastUsedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface APITokenCreateResponse {
  token: string;
  apiToken: APIToken;
}

export interface DeviceCodeVerification {
  userCode: string;
  status: 'pending' | 'approved' | 'denied' | 'expired';
  expired: boolean;
  expiresAt: string;
}

export type PreviewJobStatus = 'pending' | 'processing' | 'completed' | 'failed';

export interface PreviewJob {
  id: string;
  fileID: string;
  status: PreviewJobStatus;
  attempts: number;
  maxAttempts: number;
  lastError?: string;
  nextRetryAt?: string;
  startedAt?: string;
  completedAt?: string;
  createdAt: string;
  updatedAt: string;
  thumbnailPath?: string;
}

export interface SSOProvider {
  name: string;
  displayName: string;
  type: 'oauth' | 'oidc' | 'saml' | 'ldap';
}

export interface LinkedAccount {
  id: string;
  userID: string;
  provider: string;
  providerUserID: string;
  createdAt: string;
}
