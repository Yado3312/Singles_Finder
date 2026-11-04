import { Injectable } from '@angular/core';

export interface Profile {
  id: string;
  name: string;
  age: number;
  role: string;
  interests: string[];
  distance: string;
  lat: number;
  lng: number;
  color: string;
  initials: string;
  online: boolean;
  bio: string;
  looking: string;
  image: string;
}

export interface ProfileFilters {
  interest?: string;
  search?: string;
}

export interface Account {
  id: string;
  name: string;
  email: string;
  interests: string[];
  personality: string;
  datePace: string;
  quizComplete: boolean;
  createdAt: string;
}

export interface PersonalityAnswers {
  interests: string[];
  personality: string;
  datePace: string;
}

export class ApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
  }
}

@Injectable({ providedIn: 'root' })
export class ProfileApiService {
  private async request<T>(path: string, init?: RequestInit): Promise<T> {
    const response = await fetch(path, {
      ...init,
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json', ...init?.headers },
    });
    if (!response.ok) {
      const body: unknown = await response.json().catch(() => null);
      const message = typeof body === 'object' && body !== null && 'error' in body
        ? String(body.error)
        : `Request failed: ${response.status}`;
      throw new ApiError(message, response.status);
    }
    if (response.status === 204) return undefined as T;
    return await response.json() as T;
  }

  me(): Promise<Account> {
    return this.request<Account>('/api/auth/me');
  }

  register(input: { name: string; email: string; password: string; birthDate: string }): Promise<Account> {
    return this.request<Account>('/api/auth/register', { method: 'POST', body: JSON.stringify(input) });
  }

  login(input: { email: string; password: string }): Promise<Account> {
    return this.request<Account>('/api/auth/login', { method: 'POST', body: JSON.stringify(input) });
  }

  logout(): Promise<void> {
    return this.request<void>('/api/auth/logout', { method: 'POST' });
  }

  saveQuiz(answers: PersonalityAnswers): Promise<Account> {
    return this.request<Account>('/api/auth/quiz', { method: 'PUT', body: JSON.stringify(answers) });
  }

  updateName(name: string): Promise<Account> {
    return this.request<Account>('/api/auth/settings', { method: 'PUT', body: JSON.stringify({ name }) });
  }

  async list(filters: ProfileFilters = {}, signal?: AbortSignal): Promise<Profile[]> {
    const params = new URLSearchParams();
    if (filters.interest && filters.interest !== 'Everyone') params.set('interest', filters.interest);
    if (filters.search?.trim()) params.set('search', filters.search.trim());
    const response = await fetch(`/api/profiles?${params}`, { signal, credentials: 'same-origin' });
    if (!response.ok) throw new ApiError(`Profile request failed: ${response.status}`, response.status);
    const data: unknown = await response.json();
    if (!Array.isArray(data)) throw new Error('Invalid profile response');
    return data as Profile[];
  }

  async get(id: string, signal?: AbortSignal): Promise<Profile> {
    const response = await fetch(`/api/profiles/${encodeURIComponent(id)}`, { signal, credentials: 'same-origin' });
    if (!response.ok) throw new ApiError(`Profile request failed: ${response.status}`, response.status);
    return await response.json() as Profile;
  }
}
