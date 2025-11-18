# Platform Integration Guide

**Document**: Platform Integration Guide for Enhanced Local Connector
**Audience**: Platform Developers (Next.js/Node.js/TypeScript)
**Last Updated**: 2025-11-18
**Version**: 1.0

---

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Prerequisites](#prerequisites)
4. [Quick Start](#quick-start)
5. [gRPC Client Setup](#grpc-client-setup)
6. [User Registration Flow](#user-registration-flow)
7. [Authentication Setup Flow](#authentication-setup-flow)
8. [OAuth Integration](#oauth-integration)
9. [User Management](#user-management)
10. [Error Handling](#error-handling)
11. [Security Considerations](#security-considerations)
12. [Testing](#testing)
13. [Production Deployment](#production-deployment)
14. [Troubleshooting](#troubleshooting)

---

## Overview

This guide shows Platform developers how to integrate with the Enhanced Local Connector for Dex, enabling:

- **User Registration**: Create users programmatically via gRPC
- **Authentication Setup**: Direct users to set up their preferred auth methods
- **OAuth Login**: Integrate Dex OAuth for authentication
- **Credential Management**: Manage user passkeys, passwords, and 2FA settings

### What You'll Build

```
┌─────────────────────────────────────────────────────────────┐
│                     INTEGRATION FLOW                        │
└─────────────────────────────────────────────────────────────┘

Platform (Next.js)                    Dex Enhanced Connector
      │                                          │
      │  1. User signs up on Platform           │
      │────────────────────────────────────────>│
      │                                          │
      │  2. Create user via gRPC                │
      │────────────────────────────────────────>│
      │  ← User created (user_id returned)      │
      │                                          │
      │  3. Generate auth setup token           │
      │────────────────────────────────────────>│
      │  ← Token saved in Dex storage           │
      │                                          │
      │  4. Send email with setup link          │
      │────────────────────────────────────────>│
      │                                          │
      │  5. User clicks link, sets up auth      │
      │                                          │
      │  6. User redirected back to Platform    │
      │<─────────────────────────────────────────
      │                                          │
      │  7. User initiates login                │
      │────────────────────────────────────────>│
      │  ← OAuth login page                     │
      │                                          │
      │  8. User authenticates (passkey/password)│
      │                                          │
      │  9. OAuth callback with code            │
      │<─────────────────────────────────────────
      │                                          │
      │  10. Exchange code for tokens           │
      │────────────────────────────────────────>│
      │  ← ID token + access token              │
      │                                          │
```

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────┐
│                     PLATFORM (Next.js)                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐         ┌──────────────────┐        │
│  │  gRPC Client     │         │  OAuth Client    │        │
│  │  (User Mgmt)     │         │  (NextAuth.js)   │        │
│  └────────┬─────────┘         └────────┬─────────┘        │
│           │                            │                   │
│           │                            │                   │
└───────────┼────────────────────────────┼───────────────────┘
            │                            │
            │ gRPC (port 5557)          │ HTTPS (port 5556)
            │                            │
┌───────────▼────────────────────────────▼───────────────────┐
│                  DEX ENHANCED CONNECTOR                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────────────┐         ┌──────────────────┐        │
│  │  gRPC Server     │         │  OAuth Server    │        │
│  │  (User API)      │         │  (OIDC Provider) │        │
│  └────────┬─────────┘         └────────┬─────────┘        │
│           │                            │                   │
│           ▼                            ▼                   │
│  ┌─────────────────────────────────────────────────┐      │
│  │  Enhanced Local Connector                       │      │
│  │  ├── Passkey (WebAuthn)                         │      │
│  │  ├── Password (bcrypt)                          │      │
│  │  ├── TOTP (2FA)                                 │      │
│  │  └── Magic Link                                 │      │
│  └─────────────────────────────────────────────────┘      │
│                            │                               │
│                            ▼                               │
│  ┌─────────────────────────────────────────────────┐      │
│  │  File Storage (data/)                           │      │
│  │  ├── users/{user-id}.json                       │      │
│  │  ├── webauthn-sessions/{session-id}.json       │      │
│  │  ├── magic-link-tokens/{token}.json            │      │
│  │  └── auth-setup-tokens/{token}.json            │      │
│  └─────────────────────────────────────────────────┘      │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Communication Protocols

1. **gRPC (User Management)**
   - Port: 5557
   - Protocol: HTTP/2
   - Auth: None (internal network) or API key (production)
   - Use: Create users, set passwords, manage credentials

2. **OAuth/OIDC (Authentication)**
   - Port: 5556
   - Protocol: HTTPS
   - Auth: OAuth 2.0 + OpenID Connect
   - Use: User login, token issuance

---

## Prerequisites

### Required Software

- **Node.js**: 18.x or higher
- **TypeScript**: 5.x or higher
- **Protocol Buffers Compiler**: `protoc` (for generating gRPC code)

### Required Packages

```json
{
  "dependencies": {
    "@grpc/grpc-js": "^1.9.0",
    "@grpc/proto-loader": "^0.7.10",
    "next-auth": "^5.0.0",
    "dotenv": "^16.3.1"
  },
  "devDependencies": {
    "@types/node": "^20.0.0",
    "typescript": "^5.0.0"
  }
}
```

### Dex Configuration

Ensure Dex is configured with:

1. **Enhanced Local Connector**:
   ```yaml
   connectors:
     - type: local-enhanced
       id: local
       name: Enopax Authentication
       config:
         baseURL: https://auth.enopax.io
         passkey:
           enabled: true
           rpID: auth.enopax.io
           rpName: Enopax
           rpOrigins:
             - https://auth.enopax.io
         dataDir: /var/lib/dex/data
   ```

2. **Platform OAuth Client**:
   ```yaml
   staticClients:
     - id: platform-client
       redirectURIs:
         - https://platform.enopax.io/api/auth/callback/dex
       name: Enopax Platform
       secret: <your-client-secret>
   ```

3. **gRPC Enabled**:
   ```yaml
   grpc:
     addr: 0.0.0.0:5557
     tlsCert: /path/to/cert.pem
     tlsKey: /path/to/key.pem
   ```

---

## Quick Start

### 1. Install Dependencies

```bash
npm install @grpc/grpc-js @grpc/proto-loader next-auth dotenv
```

### 2. Copy Protobuf Definition

Copy `api/v2/api.proto` from the Dex repository to your Platform project:

```bash
mkdir -p platform/lib/dex/proto
cp dex/api/v2/api.proto platform/lib/dex/proto/
```

### 3. Create Environment Variables

```env
# .env.local

# Dex URLs
DEX_ISSUER=https://auth.enopax.io
DEX_GRPC_URL=auth.enopax.io:5557

# OAuth Client
DEX_CLIENT_ID=platform-client
DEX_CLIENT_SECRET=your-client-secret

# Platform URLs
NEXTAUTH_URL=https://platform.enopax.io
NEXTAUTH_SECRET=your-nextauth-secret
```

### 4. Create gRPC Client

```typescript
// lib/dex/grpc-client.ts

import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import path from 'path';

const PROTO_PATH = path.join(process.cwd(), 'lib/dex/proto/api.proto');

// Load protobuf
const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
});

const proto = grpc.loadPackageDefinition(packageDefinition) as any;

// Create gRPC client
export function createDexClient() {
  const client = new proto.api.EnhancedLocalConnector(
    process.env.DEX_GRPC_URL!,
    grpc.credentials.createInsecure() // Use createSsl() in production
  );

  return client;
}

export type DexClient = ReturnType<typeof createDexClient>;
```

### 5. Test Connection

```typescript
// scripts/test-dex-connection.ts

import { createDexClient } from '../lib/dex/grpc-client';

async function testConnection() {
  const client = createDexClient();

  // Test CreateUser
  client.CreateUser(
    {
      email: 'test@example.com',
      username: 'testuser',
      display_name: 'Test User',
    },
    (err: any, response: any) => {
      if (err) {
        console.error('Error:', err.message);
        return;
      }

      console.log('User created:', response.user);
    }
  );
}

testConnection();
```

Run:
```bash
npx ts-node scripts/test-dex-connection.ts
```

---

## gRPC Client Setup

### TypeScript Type Definitions

Create type definitions for better TypeScript support:

```typescript
// lib/dex/types.ts

export interface EnhancedUser {
  id: string;
  email: string;
  username: string;
  display_name: string;
  email_verified: boolean;
  has_password: boolean;
  passkey_count: number;
  totp_enabled: boolean;
  magic_link_enabled: boolean;
  require_2fa: boolean;
  created_at: string;
  updated_at: string;
  last_login_at?: string;
}

export interface Passkey {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
  transports: string[];
}

export interface CreateUserRequest {
  email: string;
  username: string;
  display_name: string;
}

export interface CreateUserResponse {
  user: EnhancedUser;
  already_exists: boolean;
}

export interface SetPasswordRequest {
  user_id: string;
  password: string;
}

export interface SetPasswordResponse {
  success: boolean;
}

// ... more type definitions
```

### Promisified gRPC Client

Wrap the callback-based gRPC client with Promises:

```typescript
// lib/dex/dex-api.ts

import { createDexClient, DexClient } from './grpc-client';
import {
  CreateUserRequest,
  CreateUserResponse,
  SetPasswordRequest,
  SetPasswordResponse,
  EnhancedUser,
  Passkey,
} from './types';

export class DexAPI {
  private client: DexClient;

  constructor() {
    this.client = createDexClient();
  }

  /**
   * Create a new user
   */
  async createUser(req: CreateUserRequest): Promise<CreateUserResponse> {
    return new Promise((resolve, reject) => {
      this.client.CreateUser(req, (err: any, response: CreateUserResponse) => {
        if (err) reject(err);
        else resolve(response);
      });
    });
  }

  /**
   * Get user by ID or email
   */
  async getUser(userId?: string, email?: string): Promise<EnhancedUser | null> {
    return new Promise((resolve, reject) => {
      this.client.GetUser(
        { user_id: userId, email },
        (err: any, response: any) => {
          if (err) {
            if (err.code === grpc.status.NOT_FOUND) {
              resolve(null);
            } else {
              reject(err);
            }
          } else if (response.not_found) {
            resolve(null);
          } else {
            resolve(response.user);
          }
        }
      );
    });
  }

  /**
   * Set user password
   */
  async setPassword(userId: string, password: string): Promise<boolean> {
    return new Promise((resolve, reject) => {
      this.client.SetPassword(
        { user_id: userId, password },
        (err: any, response: SetPasswordResponse) => {
          if (err) reject(err);
          else resolve(response.success);
        }
      );
    });
  }

  /**
   * List user's passkeys
   */
  async listPasskeys(userId: string): Promise<Passkey[]> {
    return new Promise((resolve, reject) => {
      this.client.ListPasskeys(
        { user_id: userId },
        (err: any, response: any) => {
          if (err) reject(err);
          else resolve(response.passkeys || []);
        }
      );
    });
  }

  /**
   * Get user's authentication methods
   */
  async getAuthMethods(userId: string): Promise<{
    hasPassword: boolean;
    passkeyCount: number;
    totpEnabled: boolean;
    magicLinkEnabled: boolean;
  }> {
    return new Promise((resolve, reject) => {
      this.client.GetAuthMethods(
        { user_id: userId },
        (err: any, response: any) => {
          if (err) reject(err);
          else resolve({
            hasPassword: response.has_password,
            passkeyCount: response.passkey_count,
            totpEnabled: response.totp_enabled,
            magicLinkEnabled: response.magic_link_enabled,
          });
        }
      );
    });
  }
}

// Export singleton instance
export const dexAPI = new DexAPI();
```

---

## User Registration Flow

### Complete Registration Implementation

```typescript
// app/api/auth/register/route.ts

import { NextRequest, NextResponse } from 'next/server';
import { dexAPI } from '@/lib/dex/dex-api';
import { z } from 'zod';
import crypto from 'crypto';

// Validation schema
const registerSchema = z.object({
  email: z.string().email('Invalid email address'),
  username: z.string().min(3).max(64).regex(/^[a-zA-Z][a-zA-Z0-9_-]*$/),
  displayName: z.string().min(1).max(128),
  password: z.string().min(8).max(128).optional(),
});

export async function POST(request: NextRequest) {
  try {
    // Parse request body
    const body = await request.json();

    // Validate input
    const validatedData = registerSchema.parse(body);

    // 1. Create user in Dex via gRPC
    const createUserResponse = await dexAPI.createUser({
      email: validatedData.email,
      username: validatedData.username,
      display_name: validatedData.displayName,
    });

    // Check if user already exists
    if (createUserResponse.already_exists) {
      return NextResponse.json(
        { error: 'User with this email already exists' },
        { status: 409 }
      );
    }

    const user = createUserResponse.user;

    // 2. Optionally set password (if provided)
    if (validatedData.password) {
      await dexAPI.setPassword(user.id, validatedData.password);
    }

    // 3. Generate auth setup token
    const authSetupToken = await generateAuthSetupToken(user.id, user.email);

    // 4. Save auth setup token in Dex storage
    await saveAuthSetupToken(authSetupToken);

    // 5. Send email with setup link
    await sendAuthSetupEmail(user.email, authSetupToken);

    // 6. Create user record in Platform database (optional)
    await createPlatformUser({
      id: user.id,
      email: user.email,
      username: user.username,
      displayName: user.display_name,
    });

    return NextResponse.json({
      success: true,
      userId: user.id,
      message: 'Registration successful. Please check your email to set up authentication.',
    });

  } catch (error) {
    if (error instanceof z.ZodError) {
      return NextResponse.json(
        { error: 'Validation failed', details: error.errors },
        { status: 400 }
      );
    }

    console.error('Registration error:', error);
    return NextResponse.json(
      { error: 'Registration failed' },
      { status: 500 }
    );
  }
}

/**
 * Generate auth setup token
 */
async function generateAuthSetupToken(userId: string, email: string) {
  const token = crypto.randomBytes(32).toString('base64url');
  const expiresAt = new Date(Date.now() + 24 * 60 * 60 * 1000); // 24 hours

  return {
    token,
    userId,
    email,
    createdAt: new Date(),
    expiresAt,
    used: false,
    returnURL: `${process.env.NEXTAUTH_URL}/dashboard`,
  };
}

/**
 * Save auth setup token in Dex storage
 * (You would implement a gRPC endpoint for this, or save directly to Dex's file storage)
 */
async function saveAuthSetupToken(token: any) {
  // Option 1: Save via gRPC (if you add a SaveAuthSetupToken endpoint)
  // await dexAPI.saveAuthSetupToken(token);

  // Option 2: Save to Platform database and pass token to Dex when needed
  // await db.authSetupTokens.create({ data: token });

  // For now, you can save it in Platform's database and verify it when user clicks the link
  console.log('Auth setup token generated:', token.token);
}

/**
 * Send auth setup email
 */
async function sendAuthSetupEmail(email: string, token: any) {
  const setupURL = `${process.env.DEX_ISSUER}/setup-auth?token=${token.token}`;

  // Use your email service (SendGrid, AWS SES, etc.)
  await sendEmail({
    to: email,
    subject: 'Complete Your Enopax Account Setup',
    html: `
      <h1>Welcome to Enopax!</h1>
      <p>Click the link below to set up your authentication methods:</p>
      <a href="${setupURL}" style="display: inline-block; padding: 12px 24px; background: #0070f3; color: white; text-decoration: none; border-radius: 6px;">
        Set Up Authentication
      </a>
      <p>This link expires in 24 hours.</p>
      <p>If you didn't sign up for Enopax, you can safely ignore this email.</p>
    `,
  });
}

/**
 * Create user record in Platform database
 */
async function createPlatformUser(user: any) {
  // Store user in your Platform database (PostgreSQL, etc.)
  // This is optional - you can rely on Dex's storage entirely
  // await db.users.create({ data: user });
}
```

---

## Authentication Setup Flow

### Auth Setup Token Management

The auth setup flow allows users to choose their preferred authentication method after registration.

```typescript
// lib/dex/auth-setup.ts

import { dexAPI } from './dex-api';

export interface AuthSetupToken {
  token: string;
  userId: string;
  email: string;
  createdAt: Date;
  expiresAt: Date;
  used: boolean;
  usedAt?: Date;
  returnURL: string;
}

/**
 * Validate auth setup token
 * (Called when user clicks the setup link)
 */
export async function validateAuthSetupToken(
  token: string
): Promise<AuthSetupToken | null> {
  // Retrieve token from your database or Dex storage
  const storedToken = await getStoredAuthSetupToken(token);

  if (!storedToken) {
    return null;
  }

  // Check if expired
  if (storedToken.expiresAt < new Date()) {
    return null;
  }

  // Check if already used
  if (storedToken.used) {
    return null;
  }

  return storedToken;
}

/**
 * Mark token as used
 */
export async function markAuthSetupTokenUsed(token: string) {
  // Update token in database
  await updateAuthSetupToken(token, {
    used: true,
    usedAt: new Date(),
  });
}

/**
 * Helper: Get stored token from database
 */
async function getStoredAuthSetupToken(
  token: string
): Promise<AuthSetupToken | null> {
  // Implement based on your storage (database, Dex file storage, etc.)
  // return await db.authSetupTokens.findUnique({ where: { token } });
  return null;
}

/**
 * Helper: Update token in database
 */
async function updateAuthSetupToken(token: string, data: Partial<AuthSetupToken>) {
  // await db.authSetupTokens.update({ where: { token }, data });
}
```

### Auth Setup Page

```typescript
// app/setup-auth/page.tsx

'use client';

import { useSearchParams } from 'next/navigation';
import { useEffect, useState } from 'react';

export default function AuthSetupPage() {
  const searchParams = useSearchParams();
  const token = searchParams.get('token');

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [userId, setUserId] = useState<string | null>(null);

  useEffect(() => {
    if (!token) {
      setError('Missing setup token');
      setLoading(false);
      return;
    }

    // Validate token with Platform backend
    fetch('/api/auth/validate-setup-token', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    })
      .then((res) => res.json())
      .then((data) => {
        if (data.error) {
          setError(data.error);
        } else {
          setUserId(data.userId);
          // Redirect to Dex setup page
          window.location.href = `${process.env.NEXT_PUBLIC_DEX_ISSUER}/setup-auth?token=${token}`;
        }
      })
      .catch((err) => {
        setError('Failed to validate token');
      })
      .finally(() => {
        setLoading(false);
      });
  }, [token]);

  if (loading) {
    return <div>Validating setup token...</div>;
  }

  if (error) {
    return (
      <div>
        <h1>Setup Error</h1>
        <p>{error}</p>
        <p>The setup link may have expired or been already used.</p>
      </div>
    );
  }

  return <div>Redirecting to authentication setup...</div>;
}
```

---

## OAuth Integration

### NextAuth.js Configuration

```typescript
// app/api/auth/[...nextauth]/route.ts

import NextAuth from 'next-auth';
import type { NextAuthOptions } from 'next-auth';

export const authOptions: NextAuthOptions = {
  providers: [
    {
      id: 'dex',
      name: 'Enopax',
      type: 'oauth',
      wellKnown: `${process.env.DEX_ISSUER}/.well-known/openid-configuration`,
      clientId: process.env.DEX_CLIENT_ID!,
      clientSecret: process.env.DEX_CLIENT_SECRET!,
      authorization: {
        params: {
          scope: 'openid email profile offline_access',
        },
      },
      profile(profile) {
        return {
          id: profile.sub,
          email: profile.email,
          name: profile.name,
          image: null,
        };
      },
    },
  ],

  callbacks: {
    async jwt({ token, account, profile }) {
      // Persist the OAuth access_token and user info
      if (account) {
        token.accessToken = account.access_token;
        token.idToken = account.id_token;
        token.refreshToken = account.refresh_token;
      }

      if (profile) {
        token.userId = profile.sub;
        token.email = profile.email;
        token.name = profile.name;
      }

      return token;
    },

    async session({ session, token }) {
      // Make user info available in session
      if (session.user) {
        session.user.id = token.userId as string;
        session.user.email = token.email as string;
        session.user.name = token.name as string;
      }

      return session;
    },
  },

  pages: {
    signIn: '/login',
    error: '/auth/error',
  },
};

const handler = NextAuth(authOptions);

export { handler as GET, handler as POST };
```

### Login Page

```typescript
// app/login/page.tsx

'use client';

import { signIn } from 'next-auth/react';
import { useState } from 'react';

export default function LoginPage() {
  const [loading, setLoading] = useState(false);

  const handleLogin = async () => {
    setLoading(true);

    try {
      // Initiate OAuth login with Dex
      await signIn('dex', {
        callbackUrl: '/dashboard',
      });
    } catch (error) {
      console.error('Login error:', error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center">
      <div className="max-w-md w-full space-y-8">
        <div>
          <h2 className="mt-6 text-center text-3xl font-extrabold text-gray-900">
            Sign in to Enopax
          </h2>
        </div>

        <button
          onClick={handleLogin}
          disabled={loading}
          className="w-full flex justify-center py-3 px-4 border border-transparent rounded-md shadow-sm text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 disabled:opacity-50"
        >
          {loading ? 'Redirecting...' : 'Sign in with Enopax'}
        </button>
      </div>
    </div>
  );
}
```

---

## User Management

### Get User Information

```typescript
// app/api/users/[userId]/route.ts

import { NextRequest, NextResponse } from 'next/server';
import { dexAPI } from '@/lib/dex/dex-api';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/app/api/auth/[...nextauth]/route';

export async function GET(
  request: NextRequest,
  { params }: { params: { userId: string } }
) {
  try {
    // Verify authenticated session
    const session = await getServerSession(authOptions);

    if (!session || session.user.id !== params.userId) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }

    // Get user from Dex
    const user = await dexAPI.getUser(params.userId);

    if (!user) {
      return NextResponse.json({ error: 'User not found' }, { status: 404 });
    }

    // Get authentication methods
    const authMethods = await dexAPI.getAuthMethods(user.id);

    // Get passkeys
    const passkeys = await dexAPI.listPasskeys(user.id);

    return NextResponse.json({
      user: {
        id: user.id,
        email: user.email,
        username: user.username,
        displayName: user.display_name,
        emailVerified: user.email_verified,
        createdAt: user.created_at,
        lastLoginAt: user.last_login_at,
      },
      authMethods: {
        hasPassword: authMethods.hasPassword,
        passkeyCount: authMethods.passkeyCount,
        totpEnabled: authMethods.totpEnabled,
        magicLinkEnabled: authMethods.magicLinkEnabled,
      },
      passkeys,
    });

  } catch (error) {
    console.error('Get user error:', error);
    return NextResponse.json(
      { error: 'Failed to get user information' },
      { status: 500 }
    );
  }
}
```

### Update User Profile

```typescript
// app/api/users/[userId]/profile/route.ts

import { NextRequest, NextResponse } from 'next/server';
import { dexAPI } from '@/lib/dex/dex-api';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/app/api/auth/[...nextauth]/route';
import { z } from 'zod';

const updateProfileSchema = z.object({
  displayName: z.string().min(1).max(128).optional(),
  username: z.string().min(3).max(64).optional(),
});

export async function PATCH(
  request: NextRequest,
  { params }: { params: { userId: string } }
) {
  try {
    // Verify authenticated session
    const session = await getServerSession(authOptions);

    if (!session || session.user.id !== params.userId) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }

    // Parse and validate request
    const body = await request.json();
    const validatedData = updateProfileSchema.parse(body);

    // Update user via gRPC
    await dexAPI.updateUser({
      user_id: params.userId,
      display_name: validatedData.displayName,
      username: validatedData.username,
    });

    return NextResponse.json({ success: true });

  } catch (error) {
    if (error instanceof z.ZodError) {
      return NextResponse.json(
        { error: 'Validation failed', details: error.errors },
        { status: 400 }
      );
    }

    console.error('Update profile error:', error);
    return NextResponse.json(
      { error: 'Failed to update profile' },
      { status: 500 }
    );
  }
}
```

### Change Password

```typescript
// app/api/users/[userId]/password/route.ts

import { NextRequest, NextResponse } from 'next/server';
import { dexAPI } from '@/lib/dex/dex-api';
import { getServerSession } from 'next-auth';
import { authOptions } from '@/app/api/auth/[...nextauth]/route';
import { z } from 'zod';

const changePasswordSchema = z.object({
  currentPassword: z.string(),
  newPassword: z.string().min(8).max(128),
});

export async function POST(
  request: NextRequest,
  { params }: { params: { userId: string } }
) {
  try {
    // Verify authenticated session
    const session = await getServerSession(authOptions);

    if (!session || session.user.id !== params.userId) {
      return NextResponse.json({ error: 'Unauthorized' }, { status: 401 });
    }

    // Parse and validate request
    const body = await request.json();
    const validatedData = changePasswordSchema.parse(body);

    // Verify current password (you'd need to implement this endpoint in gRPC)
    // const verified = await dexAPI.verifyPassword(params.userId, validatedData.currentPassword);
    // if (!verified) {
    //   return NextResponse.json({ error: 'Current password is incorrect' }, { status: 400 });
    // }

    // Set new password
    await dexAPI.setPassword(params.userId, validatedData.newPassword);

    return NextResponse.json({ success: true });

  } catch (error) {
    if (error instanceof z.ZodError) {
      return NextResponse.json(
        { error: 'Validation failed', details: error.errors },
        { status: 400 }
      );
    }

    console.error('Change password error:', error);
    return NextResponse.json(
      { error: 'Failed to change password' },
      { status: 500 }
    );
  }
}
```

---

## Error Handling

### gRPC Error Handling

```typescript
// lib/dex/error-handler.ts

import * as grpc from '@grpc/grpc-js';

export class DexError extends Error {
  constructor(
    message: string,
    public code: grpc.status,
    public details?: any
  ) {
    super(message);
    this.name = 'DexError';
  }
}

export function handleGrpcError(error: any): DexError {
  if (error.code === grpc.status.NOT_FOUND) {
    return new DexError('Resource not found', grpc.status.NOT_FOUND);
  }

  if (error.code === grpc.status.ALREADY_EXISTS) {
    return new DexError('Resource already exists', grpc.status.ALREADY_EXISTS);
  }

  if (error.code === grpc.status.INVALID_ARGUMENT) {
    return new DexError('Invalid argument', grpc.status.INVALID_ARGUMENT, error.details);
  }

  if (error.code === grpc.status.UNAUTHENTICATED) {
    return new DexError('Authentication required', grpc.status.UNAUTHENTICATED);
  }

  if (error.code === grpc.status.PERMISSION_DENIED) {
    return new DexError('Permission denied', grpc.status.PERMISSION_DENIED);
  }

  // Default error
  return new DexError(
    error.message || 'Unknown error',
    error.code || grpc.status.UNKNOWN
  );
}
```

### Usage in API Routes

```typescript
import { handleGrpcError, DexError } from '@/lib/dex/error-handler';
import * as grpc from '@grpc/grpc-js';

export async function POST(request: NextRequest) {
  try {
    // ... your code
    await dexAPI.createUser(data);

  } catch (error) {
    const dexError = handleGrpcError(error);

    // Map gRPC status codes to HTTP status codes
    let httpStatus = 500;

    switch (dexError.code) {
      case grpc.status.NOT_FOUND:
        httpStatus = 404;
        break;
      case grpc.status.ALREADY_EXISTS:
        httpStatus = 409;
        break;
      case grpc.status.INVALID_ARGUMENT:
        httpStatus = 400;
        break;
      case grpc.status.UNAUTHENTICATED:
        httpStatus = 401;
        break;
      case grpc.status.PERMISSION_DENIED:
        httpStatus = 403;
        break;
    }

    return NextResponse.json(
      {
        error: dexError.message,
        details: dexError.details,
      },
      { status: httpStatus }
    );
  }
}
```

---

## Security Considerations

### 1. Secure gRPC Communication

**Development (Insecure)**:
```typescript
const client = new proto.api.EnhancedLocalConnector(
  'localhost:5557',
  grpc.credentials.createInsecure()
);
```

**Production (TLS)**:
```typescript
import fs from 'fs';

const rootCert = fs.readFileSync('/path/to/ca.pem');
const privateKey = fs.readFileSync('/path/to/client-key.pem');
const certChain = fs.readFileSync('/path/to/client-cert.pem');

const credentials = grpc.credentials.createSsl(
  rootCert,
  privateKey,
  certChain
);

const client = new proto.api.EnhancedLocalConnector(
  'auth.enopax.io:5557',
  credentials
);
```

### 2. API Key Authentication (Future)

When API key authentication is implemented in Dex:

```typescript
const metadata = new grpc.Metadata();
metadata.add('x-api-key', process.env.DEX_API_KEY!);

client.CreateUser(request, metadata, (err, response) => {
  // ...
});
```

### 3. Input Validation

Always validate user input before sending to Dex:

```typescript
import { z } from 'zod';

const emailSchema = z.string().email();
const passwordSchema = z.string().min(8).max(128);
const usernameSchema = z.string().min(3).max(64).regex(/^[a-zA-Z][a-zA-Z0-9_-]*$/);

// Validate before calling Dex API
const validatedEmail = emailSchema.parse(userInput.email);
```

### 4. Rate Limiting

Implement rate limiting on your Platform API routes:

```typescript
import rateLimit from 'express-rate-limit';

const limiter = rateLimit({
  windowMs: 15 * 60 * 1000, // 15 minutes
  max: 5, // 5 requests per window
  message: 'Too many requests, please try again later',
});

// Apply to sensitive routes
app.use('/api/auth/register', limiter);
```

### 5. Session Management

Use secure session settings in NextAuth.js:

```typescript
export const authOptions: NextAuthOptions = {
  // ...
  session: {
    strategy: 'jwt',
    maxAge: 30 * 24 * 60 * 60, // 30 days
  },

  cookies: {
    sessionToken: {
      name: '__Secure-next-auth.session-token',
      options: {
        httpOnly: true,
        sameSite: 'lax',
        path: '/',
        secure: process.env.NODE_ENV === 'production',
      },
    },
  },
};
```

---

## Testing

### Unit Tests

```typescript
// __tests__/lib/dex/dex-api.test.ts

import { dexAPI } from '@/lib/dex/dex-api';

describe('DexAPI', () => {
  describe('createUser', () => {
    it('should create a new user', async () => {
      const result = await dexAPI.createUser({
        email: 'test@example.com',
        username: 'testuser',
        display_name: 'Test User',
      });

      expect(result.user).toBeDefined();
      expect(result.user.email).toBe('test@example.com');
      expect(result.already_exists).toBe(false);
    });

    it('should return already_exists for duplicate email', async () => {
      // First creation
      await dexAPI.createUser({
        email: 'duplicate@example.com',
        username: 'user1',
        display_name: 'User 1',
      });

      // Second creation with same email
      const result = await dexAPI.createUser({
        email: 'duplicate@example.com',
        username: 'user2',
        display_name: 'User 2',
      });

      expect(result.already_exists).toBe(true);
    });
  });

  describe('getUser', () => {
    it('should retrieve user by ID', async () => {
      const created = await dexAPI.createUser({
        email: 'retrieve@example.com',
        username: 'retrieveuser',
        display_name: 'Retrieve User',
      });

      const user = await dexAPI.getUser(created.user.id);

      expect(user).toBeDefined();
      expect(user!.email).toBe('retrieve@example.com');
    });

    it('should return null for non-existent user', async () => {
      const user = await dexAPI.getUser('non-existent-id');
      expect(user).toBeNull();
    });
  });
});
```

### Integration Tests

```typescript
// __tests__/api/auth/register.test.ts

import { POST } from '@/app/api/auth/register/route';
import { NextRequest } from 'next/server';

describe('/api/auth/register', () => {
  it('should register a new user', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({
        email: 'newuser@example.com',
        username: 'newuser',
        displayName: 'New User',
      }),
    });

    const response = await POST(request);
    const data = await response.json();

    expect(response.status).toBe(200);
    expect(data.success).toBe(true);
    expect(data.userId).toBeDefined();
  });

  it('should reject invalid email', async () => {
    const request = new NextRequest('http://localhost:3000/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({
        email: 'invalid-email',
        username: 'user',
        displayName: 'User',
      }),
    });

    const response = await POST(request);

    expect(response.status).toBe(400);
  });
});
```

---

## Production Deployment

### Environment Variables

Create `.env.production`:

```env
# Dex Configuration
DEX_ISSUER=https://auth.enopax.io
DEX_GRPC_URL=auth.enopax.io:5557

# OAuth Client
DEX_CLIENT_ID=platform-client-prod
DEX_CLIENT_SECRET=<your-production-secret>

# NextAuth Configuration
NEXTAUTH_URL=https://platform.enopax.io
NEXTAUTH_SECRET=<your-nextauth-secret>

# Email Configuration
SMTP_HOST=smtp.sendgrid.net
SMTP_PORT=587
SMTP_USERNAME=apikey
SMTP_PASSWORD=<your-sendgrid-api-key>
SMTP_FROM=noreply@enopax.io
```

### gRPC TLS Configuration

```typescript
// lib/dex/grpc-client.production.ts

import * as grpc from '@grpc/grpc-js';
import * as protoLoader from '@grpc/proto-loader';
import fs from 'fs';
import path from 'path';

export function createDexClient() {
  const PROTO_PATH = path.join(process.cwd(), 'lib/dex/proto/api.proto');

  const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true,
  });

  const proto = grpc.loadPackageDefinition(packageDefinition) as any;

  // Load TLS certificates
  const rootCert = fs.readFileSync(
    process.env.DEX_GRPC_CA_CERT || '/etc/ssl/certs/dex-ca.pem'
  );

  const credentials = grpc.credentials.createSsl(rootCert);

  const client = new proto.api.EnhancedLocalConnector(
    process.env.DEX_GRPC_URL!,
    credentials
  );

  return client;
}
```

### Health Checks

```typescript
// app/api/health/dex/route.ts

import { NextResponse } from 'next/server';
import { dexAPI } from '@/lib/dex/dex-api';

export async function GET() {
  try {
    // Try to get a user (or create a health check endpoint in Dex)
    // This is just a connectivity test
    await dexAPI.getUser('health-check-user');

    return NextResponse.json({ status: 'healthy', service: 'dex' });
  } catch (error) {
    return NextResponse.json(
      { status: 'unhealthy', service: 'dex', error: String(error) },
      { status: 503 }
    );
  }
}
```

---

## Troubleshooting

### Common Issues

#### 1. gRPC Connection Refused

**Error**: `Error: 14 UNAVAILABLE: Connection refused`

**Solutions**:
- Check Dex is running: `curl http://localhost:5556/healthz`
- Verify gRPC port is correct (default: 5557)
- Check firewall rules allow gRPC port
- Verify Dex gRPC is enabled in config

#### 2. OAuth Callback Error

**Error**: `redirect_uri_mismatch`

**Solutions**:
- Ensure redirect URI in Dex config matches NextAuth callback URL
- Check HTTPS is used in production
- Verify `NEXTAUTH_URL` environment variable is correct

#### 3. User Already Exists

**Error**: `User with this email already exists`

**Solutions**:
- This is expected behavior - Dex returns the existing user
- Check `already_exists` flag in response
- Handle duplicate users gracefully in your UI

#### 4. Auth Setup Token Expired

**Error**: `Invalid or expired setup token`

**Solutions**:
- Tokens expire after 24 hours by default
- Allow users to request new setup email
- Implement token refresh mechanism

#### 5. TOTP Verification Fails

**Error**: `Invalid TOTP code`

**Solutions**:
- Check time synchronization on server and client
- TOTP uses 30-second time windows
- Verify TOTP secret is correctly stored
- Allow for time drift (±1 window)

### Debug Logging

Enable debug logging in development:

```typescript
// lib/dex/dex-api.ts

export class DexAPI {
  private debug = process.env.NODE_ENV === 'development';

  async createUser(req: CreateUserRequest): Promise<CreateUserResponse> {
    if (this.debug) {
      console.log('[DexAPI] CreateUser request:', req);
    }

    return new Promise((resolve, reject) => {
      this.client.CreateUser(req, (err, response) => {
        if (this.debug) {
          console.log('[DexAPI] CreateUser response:', { err, response });
        }

        if (err) reject(err);
        else resolve(response);
      });
    });
  }
}
```

---

## Next Steps

After completing this integration:

1. **Test thoroughly**:
   - Unit tests for gRPC client
   - Integration tests for registration flow
   - End-to-end tests for OAuth login

2. **Monitor**:
   - Set up error tracking (Sentry, DataDog)
   - Monitor gRPC latency
   - Track authentication success/failure rates

3. **Scale**:
   - Add connection pooling for gRPC
   - Implement caching for user data
   - Load test authentication flows

4. **Enhance**:
   - Add social login providers (GitHub, Google)
   - Implement account recovery flows
   - Add admin dashboard for user management

---

## Support

### Resources

- **Enhanced Local Connector Docs**: `/docs/enhancements/`
- **gRPC API Reference**: `/docs/enhancements/grpc-api.md`
- **Authentication Flows**: `/docs/enhancements/authentication-flows.md`
- **Dex Documentation**: https://dexidp.io/docs/

### Getting Help

If you encounter issues:

1. Check the [Troubleshooting](#troubleshooting) section
2. Review Dex logs: `journalctl -u dex -f`
3. Enable debug logging in Platform
4. Open an issue in the Dex repository

---

**Last Updated**: 2025-11-18
**Version**: 1.0
**Author**: Enopax Platform Team
