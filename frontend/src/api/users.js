import { authenticatedRequest } from './auth';

const USERS_PATH = '/api/v1/users';

export function getMyProfile() {
  return authenticatedRequest(`${USERS_PATH}/me`);
}

export function updateMyProfile(changes) {
  return authenticatedRequest(`${USERS_PATH}/me`, {
    method: 'PATCH',
    body: JSON.stringify(changes),
  });
}

export function updateMyAvatar(mediaId) {
  return authenticatedRequest(`${USERS_PATH}/me/avatar`, {
    method: 'PUT',
    body: JSON.stringify({ media_id: mediaId }),
  });
}

export async function searchUsers(filters) {
  const query = new URLSearchParams();
  Object.entries(filters).forEach(([key, value]) => {
    if (value !== '' && value !== null && value !== undefined) query.set(key, String(value));
  });

  const payload = await authenticatedRequest(`${USERS_PATH}?${query.toString()}`, {
    returnEnvelope: true,
  });

  return {
    users: Array.isArray(payload.data) ? payload.data : [],
    meta: {
      next_cursor: payload.meta?.next_cursor ?? null,
      has_more: Boolean(payload.meta?.has_more),
      limit: Number(payload.meta?.limit) || Number(filters.limit) || 20,
    },
  };
}
