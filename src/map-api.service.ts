import { Injectable } from '@angular/core';

export interface MapRequest {
  id: string;
  creatorName: string;
  creatorInterests: string[];
  description: string;
  lat: number;
  lng: number;
  createdAt: string;
  participantCount: number;
  joinedByMe: boolean;
  isCreator: boolean;
}

export interface NearbyPlace { id: string; name: string; category: string; lat: number; lng: number; distanceMeters: number; }

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
export class MapApiService {
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

  listRequests(signal?: AbortSignal): Promise<MapRequest[]> {
    return this.request<MapRequest[]>('/api/requests', { signal });
  }

  createRequest(input: { description: string; lat: number; lng: number }): Promise<MapRequest> {
    return this.request<MapRequest>('/api/requests', { method: 'POST', body: JSON.stringify(input) });
  }

  joinRequest(id: string): Promise<MapRequest> {
    return this.request<MapRequest>(`/api/requests/${encodeURIComponent(id)}/join`, { method: 'POST' });
  }

  nearbyPlaces(lat: number, lng: number): Promise<NearbyPlace[]> {
    const query = new URLSearchParams({ lat: String(lat), lng: String(lng) });
    return this.request<NearbyPlace[]>(`/api/places?${query}`);
  }
}
