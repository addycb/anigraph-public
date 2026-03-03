/**
 * CSRF Token Management Composable
 * Automatically fetches and includes CSRF token in authenticated requests
 */

import { ref } from 'vue'
import { api } from '@/utils/api'

const csrfToken = ref<string | null>(null)

export const useCsrf = () => {
  /**
   * Fetch CSRF token from server and cache it
   */
  const fetchCsrfToken = async (): Promise<string> => {
    try {
      const response = await api<{ token: string }>('/auth/csrf-token');
      csrfToken.value = response.token;
      return response.token;
    } catch (error) {
      console.error('Failed to fetch CSRF token:', error);
      throw error;
    }
  };

  /**
   * Get CSRF token, fetching if not already cached
   */
  const getCsrfToken = async (): Promise<string> => {
    if (csrfToken.value) {
      return csrfToken.value;
    }
    return await fetchCsrfToken();
  };

  /**
   * Get headers object with CSRF token
   */
  const getCsrfHeaders = async (): Promise<Record<string, string>> => {
    const token = await getCsrfToken();
    return {
      'X-CSRF-Token': token,
    };
  };

  /**
   * Make an authenticated POST request with CSRF protection
   */
  const csrfPost = async <T = any>(url: string, body?: any): Promise<T> => {
    const headers = await getCsrfHeaders();
    const apiPath = url.replace(/^\/api/, '');
    return await api<T>(apiPath, {
      method: 'POST',
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  };

  /**
   * Make an authenticated PATCH request with CSRF protection
   */
  const csrfPatch = async <T = any>(url: string, body?: any): Promise<T> => {
    const headers = await getCsrfHeaders();
    const apiPath = url.replace(/^\/api/, '');
    return await api<T>(apiPath, {
      method: 'PATCH',
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  };

  /**
   * Make an authenticated DELETE request with CSRF protection
   */
  const csrfDelete = async <T = any>(url: string, body?: any): Promise<T> => {
    const headers = await getCsrfHeaders();
    const apiPath = url.replace(/^\/api/, '');
    return await api<T>(apiPath, {
      method: 'DELETE',
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });
  };

  return {
    csrfToken,
    fetchCsrfToken,
    getCsrfToken,
    getCsrfHeaders,
    csrfPost,
    csrfPatch,
    csrfDelete,
  };
};
