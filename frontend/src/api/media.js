import { AuthApiError, authenticatedRequest } from './auth';

const MEDIA_PATH = '/api/v1/media/uploads';

export function requestMediaUpload(file) {
  return authenticatedRequest(MEDIA_PATH, {
    method: 'POST',
    body: JSON.stringify({
      type: 'image',
      filename: file.name,
      mime_type: file.type,
      size: file.size,
    }),
  });
}

export async function uploadMediaFile(file, upload) {
  if (!upload?.url || !upload?.method) {
    throw new AuthApiError('服务端没有返回有效的上传地址', 'INVALID_UPLOAD_RESPONSE');
  }

  let response;
  try {
    response = await fetch(upload.url, {
      method: upload.method,
      headers: upload.headers || {},
      body: file,
    });
  } catch {
    throw new AuthApiError('图片上传失败，请检查网络后重试', 'MEDIA_UPLOAD_FAILED');
  }

  if (!response.ok) {
    throw new AuthApiError(`图片上传失败（${response.status}）`, 'MEDIA_UPLOAD_FAILED', response.status);
  }
}

export function completeMediaUpload(mediaId) {
  const normalizedId = Number(mediaId);
  if (!Number.isSafeInteger(normalizedId) || normalizedId <= 0) {
    throw new AuthApiError('服务端没有返回有效的媒体 ID', 'INVALID_UPLOAD_RESPONSE');
  }
  return authenticatedRequest(`${MEDIA_PATH}/${normalizedId}/complete`, {
    method: 'POST',
  });
}
