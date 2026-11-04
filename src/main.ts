import 'zone.js';
import { bootstrapApplication } from '@angular/platform-browser';
import { Component, AfterViewInit, OnDestroy, inject, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import * as L from 'leaflet';
import { Account, ApiError, PersonalityAnswers, Profile, ProfileApiService } from './profile-api.service';

const interestOptions = [
  'Nature', 'Music', 'Art', 'Coffee', 'Travel', 'Cooking',
  'Reading', 'Sports', 'Video games', 'Animals', 'Movies', 'Volunteering',
];

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [CommonModule],
  templateUrl: './app.component.html',
  styleUrl: './app.component.css',
})
export class AppComponent implements AfterViewInit, OnDestroy {
  private readonly api = inject(ProfileApiService);
  readonly page = signal<'loading' | 'auth' | 'quiz' | 'app'>('loading');
  readonly authMode = signal<'login' | 'register'>('login');
  readonly activeTab = signal<'map' | 'settings'>('map');
  readonly account = signal<Account | null>(null);
  readonly authBusy = signal(false);
  readonly error = signal('');
  readonly toast = signal('');
  readonly interestOptions = interestOptions;
  readonly selectedInterests = signal<string[]>([]);
  readonly personality = signal('');
  readonly datePace = signal('');
  readonly people = signal<Profile[]>([]);
  readonly selected = signal<Profile | null>(null);
  readonly loadingProfiles = signal(false);
  readonly search = signal('');
  readonly profileError = signal('');
  readonly settingsName = signal('');
  readonly savingSettings = signal(false);
  readonly filtered = computed(() => this.people());
  private map?: L.Map;
  private markers: L.Marker[] = [];
  private userMarker?: L.CircleMarker;
  private toastTimer?: ReturnType<typeof setTimeout>;
  private searchTimer?: ReturnType<typeof setTimeout>;
  private requestId = 0;
  private request?: AbortController;

  ngAfterViewInit() {
    void this.restoreSession();
  }

  ngOnDestroy() {
    this.request?.abort();
    this.destroyMap();
    if (this.toastTimer) clearTimeout(this.toastTimer);
    if (this.searchTimer) clearTimeout(this.searchTimer);
  }

  private async restoreSession() {
    try {
      this.setAccount(await this.api.me());
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        this.page.set('auth');
      } else {
        this.page.set('auth');
        this.error.set('Could not reach the server. Make sure the API and database are running.');
      }
    }
  }

  setAuthMode(mode: 'login' | 'register') {
    this.authMode.set(mode);
    this.error.set('');
  }

  async submitAuth(event: Event) {
    event.preventDefault();
    const form = event.target as HTMLFormElement;
    const data = new FormData(form);
    this.authBusy.set(true);
    this.error.set('');
    try {
      const email = String(data.get('email') ?? '');
      const password = String(data.get('password') ?? '');
      const account = this.authMode() === 'register'
        ? await this.api.register({
          name: String(data.get('name') ?? ''),
          email,
          password,
          birthDate: String(data.get('birthDate') ?? ''),
        })
        : await this.api.login({ email, password });
      form.reset();
      this.setAccount(account);
    } catch (error) {
      this.error.set(this.errorMessage(error, 'We could not complete your request. Please try again.'));
    } finally {
      this.authBusy.set(false);
    }
  }

  private setAccount(account: Account) {
    this.account.set(account);
    this.settingsName.set(account.name);
    this.selectedInterests.set(account.interests ?? []);
    this.page.set(account.quizComplete ? 'app' : 'quiz');
    if (account.quizComplete) this.enterMap();
  }

  toggleInterest(interest: string) {
    this.selectedInterests.update(current => current.includes(interest)
      ? current.filter(item => item !== interest)
      : current.length < 8 ? [...current, interest] : current);
  }

  async submitQuiz() {
    if (!this.quizValid()) return;
    this.authBusy.set(true);
    this.error.set('');
    const answers: PersonalityAnswers = {
      interests: this.selectedInterests(),
      personality: this.personality(),
      datePace: this.datePace(),
    };
    try {
      this.setAccount(await this.api.saveQuiz(answers));
    } catch (error) {
      this.error.set(this.errorMessage(error, 'We could not save your answers. Please try again.'));
    } finally {
      this.authBusy.set(false);
    }
  }

  quizValid() {
    return this.selectedInterests().length >= 3 && !!this.personality() && !!this.datePace();
  }

  async saveSettings() {
    this.savingSettings.set(true);
    this.error.set('');
    try {
      const updated = await this.api.updateName(this.settingsName());
      this.account.set(updated);
      this.showToast('Your name has been saved.');
    } catch (error) {
      this.error.set(this.errorMessage(error, 'We could not save your changes. Please try again.'));
    } finally {
      this.savingSettings.set(false);
    }
  }

  editPersonalityTest() {
    const current = this.account();
    if (!current) return;
    this.selectedInterests.set(current.interests);
    this.personality.set(current.personality);
    this.datePace.set(current.datePace);
    this.page.set('quiz');
    this.destroyMap();
  }

  async logout() {
    try {
      await this.api.logout();
      this.account.set(null);
      this.page.set('auth');
      this.activeTab.set('map');
      this.error.set('');
      this.people.set([]);
      this.selected.set(null);
      this.destroyMap();
    } catch (error) {
      this.error.set(this.errorMessage(error, 'We could not sign you out. Please try again.'));
    }
  }

  openTab(tab: 'map' | 'settings') {
    this.activeTab.set(tab);
    this.error.set('');
    if (tab === 'map') this.enterMap();
    else this.destroyMap();
  }

  private enterMap() {
    this.activeTab.set('map');
    setTimeout(() => {
      this.initMap();
      this.loadProfiles();
    }, 0);
  }

  private initMap() {
    const element = document.getElementById('map');
    if (!element || this.page() !== 'app' || this.activeTab() !== 'map') return;
    this.destroyMap();
    this.map = L.map(element, { zoomControl: false, scrollWheelZoom: false })
      .setView([28.6345, -106.0810], 12);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap contributors',
      maxZoom: 19,
    }).addTo(this.map);
    L.control.zoom({ position: 'bottomright' }).addTo(this.map);
    this.renderMarkers();
    this.locateUser();
  }

  private destroyMap() {
    this.map?.remove();
    this.map = undefined;
    this.markers = [];
    this.userMarker = undefined;
  }

  private locateUser() {
    if (!navigator.geolocation) {
      this.showToast('Location is not available in this browser. You can still explore the map.');
      return;
    }
    navigator.geolocation.getCurrentPosition(position => {
      if (!this.map) return;
      const { latitude, longitude } = position.coords;
      this.map.setView([latitude, longitude], 13);
      this.userMarker?.remove();
      this.userMarker = L.circleMarker([latitude, longitude], {
        radius: 9,
        color: '#ffffff',
        weight: 3,
        fillColor: '#c87f62',
        fillOpacity: 1,
      }).addTo(this.map);
      this.userMarker.bindPopup('Your location on this device. It is never stored or shared.');
    }, () => {
      this.showToast('Location permission was not granted. The map is still yours to explore.');
    }, { enableHighAccuracy: false, timeout: 10000, maximumAge: 300000 });
  }

  async loadProfiles() {
    const currentRequest = ++this.requestId;
    this.request?.abort();
    this.request = new AbortController();
    this.loadingProfiles.set(true);
    this.profileError.set('');
    try {
      const profiles = await this.api.list({ search: this.search() }, this.request.signal);
      if (currentRequest !== this.requestId) return;
      this.people.set(profiles);
      if (!profiles.length) this.selected.set(null);
      else if (!profiles.some(profile => profile.id === this.selected()?.id)) this.selected.set(profiles[0]);
      this.renderMarkers();
    } catch (error) {
      if (currentRequest !== this.requestId) return;
      this.people.set([]);
      this.selected.set(null);
      this.profileError.set(this.errorMessage(error, 'We could not load profiles. Please try again.'));
      this.renderMarkers();
    } finally {
      if (currentRequest === this.requestId) this.loadingProfiles.set(false);
    }
  }

  setSearch(value: string) {
    this.search.set(value);
    if (this.searchTimer) clearTimeout(this.searchTimer);
    this.searchTimer = setTimeout(() => void this.loadProfiles(), 250);
  }

  private renderMarkers() {
    if (!this.map) return;
    this.markers.forEach(marker => marker.remove());
    this.markers = this.filtered().map(profile => {
      const icon = L.divIcon({
        className: 'person-marker',
        html: `<div class="marker-shell ${this.selected()?.id === profile.id ? 'is-selected' : ''}"><img src="${this.escapeAttribute(profile.image)}" alt=""></div>`,
        iconSize: [62, 62],
        iconAnchor: [31, 31],
      });
      const marker = L.marker([profile.lat, profile.lng], { icon }).addTo(this.map!);
      marker.on('click', () => this.select(profile));
      return marker;
    });
  }

  private escapeAttribute(value: string) {
    return value.replace(/[&"<>']/g, character => ({
      '&': '&amp;', '"': '&quot;', '<': '&lt;', '>': '&gt;', "'": '&#39;',
    }[character] ?? character));
  }

  select(profile: Profile) {
    this.selected.set(profile);
    this.renderMarkers();
    this.map?.panTo([profile.lat, profile.lng], { animate: true });
  }

  showToast(message: string) {
    this.toast.set(message);
    if (this.toastTimer) clearTimeout(this.toastTimer);
    this.toastTimer = setTimeout(() => this.toast.set(''), 4000);
  }

  private errorMessage(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 409) return 'An account with that email already exists.';
      if (error.status === 401) return 'That email and password do not match.';
      if (error.status === 400) return error.message;
      if (error.status >= 500) return 'The server could not complete your request. Please try again later.';
    }
    return fallback;
  }
}

bootstrapApplication(AppComponent).catch(error => console.error(error));
