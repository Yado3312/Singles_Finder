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

@Injectable({ providedIn: 'root' })
export class ProfileApiService {
  async list(filters: ProfileFilters = {}, signal?: AbortSignal): Promise<Profile[]> {
    const params = new URLSearchParams();
    if (filters.interest && filters.interest !== 'Everyone') params.set('interest', filters.interest);
    if (filters.search?.trim()) params.set('search', filters.search.trim());
    const response = await fetch(`/api/profiles?${params}`, { signal });
    if (!response.ok) throw new Error(`Profile request failed: ${response.status}`);
    const data: unknown = await response.json();
    if (!Array.isArray(data)) throw new Error('Invalid profile response');
    return data as Profile[];
  }

  async get(id: string, signal?: AbortSignal): Promise<Profile> {
    const response = await fetch(`/api/profiles/${encodeURIComponent(id)}`, { signal });
    if (!response.ok) throw new Error(`Profile request failed: ${response.status}`);
    return await response.json() as Profile;
  }
}
