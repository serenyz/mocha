import { AuthApiError, authenticatedRequest } from './auth';

const CONVERSATIONS_PATH = '/api/v1/conversations';

function buildQuery(values) {
  const query = new URLSearchParams();
  Object.entries(values).forEach(([key, value]) => {
    if (value !== '' && value !== null && value !== undefined) {
      query.set(key, String(value));
    }
  });
  const value = query.toString();
  return value ? `?${value}` : '';
}

function positiveInteger(value, label) {
  const normalized = Number(value);
  if (!Number.isSafeInteger(normalized) || normalized <= 0) {
    throw new AuthApiError(`${label}不正确`, 'INVALID_ARGUMENT');
  }
  return normalized;
}

function sequence(value, label) {
  if (value === undefined || value === null || value === '') return undefined;
  const normalized = Number(value);
  if (!Number.isSafeInteger(normalized) || normalized < 0) {
    throw new AuthApiError(`${label}不正确`, 'INVALID_ARGUMENT');
  }
  return normalized;
}

function pageLimit(value, maximum) {
  if (value === undefined || value === null || value === '') return undefined;
  const normalized = Number(value);
  if (!Number.isSafeInteger(normalized) || normalized < 1 || normalized > maximum) {
    throw new AuthApiError(`分页数量需为 1–${maximum} 的整数`, 'INVALID_ARGUMENT');
  }
  return normalized;
}

export function createDirectConversation(userId) {
  return authenticatedRequest(`${CONVERSATIONS_PATH}/direct`, {
    method: 'POST',
    body: JSON.stringify({ user_id: positiveInteger(userId, '用户 ID') }),
  });
}

export function createGroupConversation({ name, avatarMediaId, userIds = [] } = {}) {
  if (!Array.isArray(userIds)) {
    throw new AuthApiError('群成员参数不正确', 'INVALID_ARGUMENT');
  }
  const payload = {
    name,
    user_ids: userIds.map((userId) => positiveInteger(userId, '用户 ID')),
  };
  if (avatarMediaId !== undefined && avatarMediaId !== null && avatarMediaId !== '') {
    payload.avatar_media_id = positiveInteger(avatarMediaId, '媒体 ID');
  }

  return authenticatedRequest(`${CONVERSATIONS_PATH}/group`, {
    method: 'POST',
    body: JSON.stringify(payload),
  });
}

export function listConversations({ cursor, limit } = {}) {
  return authenticatedRequest(`${CONVERSATIONS_PATH}${buildQuery({
    cursor,
    limit: pageLimit(limit, 50),
  })}`);
}

export function getConversation(conversationId) {
  return authenticatedRequest(`${CONVERSATIONS_PATH}/${positiveInteger(conversationId, '会话 ID')}`);
}

export function listMessages(conversationId, { beforeSeq, afterSeq, limit } = {}) {
  const normalizedBeforeSeq = sequence(beforeSeq, 'before_seq');
  const normalizedAfterSeq = sequence(afterSeq, 'after_seq');
  if (normalizedBeforeSeq !== undefined && normalizedAfterSeq !== undefined) {
    throw new AuthApiError('before_seq 和 after_seq 不能同时使用', 'INVALID_ARGUMENT');
  }

  return authenticatedRequest(
    `${CONVERSATIONS_PATH}/${positiveInteger(conversationId, '会话 ID')}/messages${buildQuery({
      before_seq: normalizedBeforeSeq,
      after_seq: normalizedAfterSeq,
      limit: pageLimit(limit, 100),
    })}`,
  );
}
