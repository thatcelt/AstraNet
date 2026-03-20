# GoMino API Keys - Developer Guide

## 📋 Overview

GoMino implements a **Developer API Key** system that allows you to:
- ✅ Bypass DPoP and Signature validation for development and testing
- ✅ Create multiple keys with different rate limits
- ✅ Integrate third-party applications and bots
- ✅ Track API usage per key

## 🚀 Quick Start

### 1. Get Your Bootstrap Key

When you first start GoMino, a bootstrap API key is automatically created:

```bash
cd /path/to/GoMino
go run cmd/api/main.go
```

The bootstrap key will be saved to `bootstrap_api_key.txt` in your project directory.

**⚠️ Important:** This key is only shown once! Save it securely.

### 2. Using the Bootstrap Key

#### Python Example:

```python
from AminoLightPy import Client

# Use API key to bypass security checks
client = Client(socket_enabled=False, api_key="gm_YOUR_KEY_HERE")

# Now login works without DPoP/Signature issues!
client.login_email("email@example.com", "password")
```

#### HTTP Request Example:

```bash
curl -X GET "http://localhost:8080/api/v1/g/s/user-profile/USER_ID" \
  -H "X-API-Key: gm_YOUR_KEY_HERE"
```

Or using Bearer token:

```bash
curl -X GET "http://localhost:8080/api/v1/g/s/user-profile/USER_ID" \
  -H "Authorization: Bearer gm_YOUR_KEY_HERE"
```

## 📚 API Endpoints

All API key management endpoints require authentication (NDCAUTH header with valid SID).

### Create API Key

**POST** `/api/v1/g/s/developer/api-keys`

Create a new API key for the authenticated user.

**Headers:**
- `NDCAUTH: sid=YOUR_SID`
- `AUID: YOUR_USER_ID`
- `X-API-Key: BOOTSTRAP_KEY` (optional, to bypass security)
- `Content-Type: application/json`

**Request Body:**
```json
{
  "name": "My Python Bot",
  "scopes": ["read", "write"],
  "rateLimit": 5000,
  "expiresInDays": 30,
  "timestamp": 1234567890000
}
```

**Response:**
```json
{
  "api:statuscode": 0,
  "api:message": "API key created successfully...",
  "apiKey": "gm_1234567890abcdef...",
  "keyInfo": {
    "id": "abc123",
    "name": "My Python Bot",
    "scopes": ["read", "write"],
    "rateLimit": 5000,
    "isActive": true,
    "createdAt": "2025-01-04T12:00:00Z",
    "expiresAt": "2025-02-03T12:00:00Z",
    "requestCount": 0
  }
}
```

### List API Keys

**GET** `/api/v1/g/s/developer/api-keys`

List all API keys for the authenticated user.

**Response:**
```json
{
  "api:statuscode": 0,
  "apiKeys": [
    {
      "id": "abc123",
      "name": "My Python Bot",
      "scopes": ["read", "write"],
      "rateLimit": 5000,
      "isActive": true,
      "createdAt": "2025-01-04T12:00:00Z",
      "requestCount": 142
    }
  ],
  "count": 1
}
```

### Get API Key Details

**GET** `/api/v1/g/s/developer/api-keys/{keyId}`

Get details for a specific API key.

### Update API Key

**PATCH** `/api/v1/g/s/developer/api-keys/{keyId}`

Update API key properties (name, rate limit, active status).

**Request Body:**
```json
{
  "name": "Updated Bot Name",
  "rateLimit": 7500,
  "isActive": true,
  "timestamp": 1234567890000
}
```

### Revoke API Key

**DELETE** `/api/v1/g/s/developer/api-keys/{keyId}`

Permanently revoke (delete) an API key.

## 🔐 Security Features

### 1. **Rate Limiting**

Each API key has a configurable rate limit (requests per hour):
- Default: 1000 requests/hour
- Bootstrap key: 10,000 requests/hour
- Custom keys: Set when creating

When rate limit is exceeded, you'll receive:
```json
{
  "api:statuscode": 429,
  "api:message": "API key rate limit exceeded"
}
```

### 2. **Key Expiration**

Keys can be set to expire after a certain number of days:
```json
{
  "expiresInDays": 30  // Expires in 30 days
}
```

Set to `0` or omit for keys that never expire.

### 3. **Key Revocation**

Keys can be revoked (deleted) at any time:
- Owner can revoke their own keys
- Admin can revoke any key

### 4. **Usage Tracking**

Every request made with an API key is tracked:
- Total request count
- Last used timestamp
- Hourly usage windows for rate limiting

## 🛠️ Python Library Integration

The AminoLightPy library has been updated to support API keys:

```python
from AminoLightPy import Client

# Method 1: Use API key from the start
client = Client(
    socket_enabled=False,
    api_key="gm_YOUR_KEY_HERE"
)

# Method 2: Set API key after initialization
client = Client(socket_enabled=False)
client.session.headers["X-API-Key"] = "gm_YOUR_KEY_HERE"

# Now all requests bypass DPoP/Signature validation
client.login_email("email@example.com", "password")
```

## 📝 Best Practices

### 1. **Key Management**

- ✅ **DO** use different keys for different applications
- ✅ **DO** set appropriate rate limits for each key
- ✅ **DO** use expiration dates for temporary access
- ✅ **DO** revoke keys immediately if compromised
- ❌ **DON'T** share API keys publicly
- ❌ **DON'T** hardcode keys in your source code (use environment variables)

### 2. **Security**

```python
# Good - Use environment variables
import os
API_KEY = os.getenv("GOMINO_API_KEY")

# Bad - Hardcoded key
API_KEY = "gm_1234567890..."  # ❌ Don't do this!
```

### 3. **Rate Limiting**

- Set realistic rate limits based on your use case
- Monitor your usage to avoid hitting limits
- Implement exponential backoff on 429 errors

### 4. **Error Handling**

```python
import requests

headers = {"X-API-Key": API_KEY}

try:
    response = requests.get(url, headers=headers)
    response.raise_for_status()
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 429:
        print("Rate limit exceeded, backing off...")
    elif e.response.status_code == 401:
        print("Invalid or expired API key")
```

## 🔄 Migration from Old System

If you were using the old system without API keys:

### Before:
```python
client = Client(socket_enabled=False)
client.login_email("email", "password")  # May fail with signature errors
```

### After:
```python
client = Client(socket_enabled=False, api_key=YOUR_API_KEY)
client.login_email("email", "password")  # Works without security issues!
```

## 🐛 Troubleshooting

### Issue: "Invalid or missing API key"

**Cause:** The key is invalid, expired, or inactive.

**Solution:**
1. Check that the key is correctly copied (no extra spaces)
2. Verify the key hasn't expired
3. Ensure the key is active (not revoked)
4. Create a new key if necessary

### Issue: "API key rate limit exceeded"

**Cause:** You've exceeded the hourly request limit.

**Solution:**
1. Wait until the next hour window
2. Create a new key with a higher rate limit
3. Implement request caching to reduce API calls

### Issue: Bootstrap key file not found

**Cause:** The server hasn't been started yet, or keys already exist.

**Solution:**
1. Start the GoMino server: `go run cmd/api/main.go`
2. If keys exist, the bootstrap won't be created (check logs)
3. Use an existing key or create a new one via API

## 📊 Key Format

API keys have the following format:
```
gm_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef
│  └─────────────────────────── 64 hex characters ──────────────────────────┘
└── Prefix (GoMino)
```

- **Prefix:** `gm_` (identifies GoMino keys)
- **Length:** 67 characters total (3 prefix + 64 hex)
- **Encoding:** Hexadecimal
- **Storage:** SHA256 hash (keys are never stored in plain text)

## 🎯 Common Use Cases

### 1. Development/Testing
```python
# Use bootstrap key for local development
client = Client(api_key=BOOTSTRAP_KEY)
```

### 2. Production Bot
```python
# Create dedicated key with appropriate limits
# POST /api/v1/g/s/developer/api-keys
{
  "name": "Production Bot",
  "rateLimit": 5000,
  "expiresInDays": 365
}
```

### 3. Temporary Access
```python
# Create short-lived key for testing
{
  "name": "Test Integration",
  "rateLimit": 100,
  "expiresInDays": 7
}
```

## 📞 Support

For issues or questions:
- GitHub Issues: https://github.com/AugustLigh/GoMino/issues
- Documentation: Check `/docs` directory

---

**Generated by GoMino** - Developer API Key System v1.0
