import 'zone.js';
import { bootstrapApplication } from '@angular/platform-browser';
import { Component, AfterViewInit, OnDestroy, inject, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import * as L from 'leaflet';
import { Account, ApiError, MapApiService, MapRequest, PersonalityAnswers } from './map-api.service';

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
  private readonly api = inject(MapApiService);
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
  readonly mapRequests = signal<MapRequest[]>([]);
  readonly selectedRequest = signal<MapRequest | null>(null);
  readonly requestDescription = signal('');
  readonly requestLocation = signal<{ lat: number; lng: number } | null>(null);
  readonly requestFormOpen = signal(false);
  readonly loadingRequests = signal(false);
  readonly requestError = signal('');
  readonly savingRequest = signal(false);
  readonly settingsName = signal('');
  readonly savingSettings = signal(false);
  private map?: L.Map;
  private markers: L.Marker[] = [];
  private locationMarker?: L.CircleMarker;
  private toastTimer?: ReturnType<typeof setTimeout>;
  private requestTimer?: ReturnType<typeof setInterval>;
  private loadingRequestList = false;
  private hasFittedRequests = false;

  ngAfterViewInit() {
    void this.restoreSession();
  }

  ngOnDestroy() {
    this.destroyMap();
    if (this.toastTimer) clearTimeout(this.toastTimer);
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
      this.mapRequests.set([]);
      this.selectedRequest.set(null);
      this.requestDescription.set('');
      this.requestLocation.set(null);
      this.requestFormOpen.set(false);
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
      void this.loadRequests();
      this.requestTimer = setInterval(() => void this.loadRequests(), 5000);
    }, 0);
  }

  private initMap() {
    const element = document.getElementById('map');
    if (!element || this.page() !== 'app' || this.activeTab() !== 'map') return;
    this.destroyMap();
    this.hasFittedRequests = false;
    this.map = L.map(element, { zoomControl: false, scrollWheelZoom: true, minZoom: 2 })
      .setView([20, 0], 2);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
      attribution: '&copy; OpenStreetMap contributors',
      maxZoom: 19,
    }).addTo(this.map);
    L.control.zoom({ position: 'bottomright' }).addTo(this.map);
    this.map.on('click', event => {
      this.requestLocation.set({
        lat: Math.round(event.latlng.lat * 1000) / 1000,
        lng: Math.round(event.latlng.lng * 1000) / 1000,
      });
      this.renderLocationMarker();
      if (!this.requestFormOpen()) this.requestFormOpen.set(true);
    });
    this.locateUser();
  }

  private destroyMap() {
    if (this.requestTimer) clearInterval(this.requestTimer);
    this.requestTimer = undefined;
    this.map?.remove();
    this.map = undefined;
    this.markers = [];
    this.locationMarker = undefined;
  }

  private locateUser() {
    if (!navigator.geolocation) {
      this.showToast('Location is not available in this browser. You can still explore the map.');
      return;
    }
    navigator.geolocation.getCurrentPosition(position => {
      if (!this.map || this.mapRequests().length) return;
      const { latitude, longitude } = position.coords;
      this.map.setView([latitude, longitude], 13);
    }, () => {
      this.showToast('No se concedió el permiso de ubicación. Puedes explorar el mapa y elegir un punto manualmente.');
    }, { enableHighAccuracy: false, timeout: 10000, maximumAge: 300000 });
  }

  async loadRequests() {
    if (this.loadingRequestList) return;
    this.loadingRequestList = true;
    this.loadingRequests.set(true);
    try {
      const requests = await this.api.listRequests();
      this.mapRequests.set(requests);
      const selectedID = this.selectedRequest()?.id;
      this.selectedRequest.set(requests.find(item => item.id === selectedID) ?? null);
      this.renderMarkers();
      if (!this.hasFittedRequests && requests.length) {
        this.fitRequests(requests);
        this.hasFittedRequests = true;
      }
      this.requestError.set('');
    } catch (error) {
      this.requestError.set(this.errorMessage(error, 'No se pudieron cargar las solicitudes. Inténtalo de nuevo.'));
    } finally {
      this.loadingRequestList = false;
      this.loadingRequests.set(false);
    }
  }

  toggleRequestForm() {
    this.requestFormOpen.update(open => !open);
    this.requestError.set('');
    if (this.requestFormOpen() && !this.requestLocation() && this.map) {
      const center = this.map.getCenter();
      this.requestLocation.set({
        lat: Math.round(center.lat * 1000) / 1000,
        lng: Math.round(center.lng * 1000) / 1000,
      });
      this.renderLocationMarker();
    }
  }

  async createMapRequest() {
    const location = this.requestLocation();
    if (!location) {
      this.requestError.set('Selecciona un punto en el mapa para ubicar la solicitud.');
      return;
    }
    this.savingRequest.set(true);
    this.requestError.set('');
    try {
      const created = await this.api.createRequest({
        description: this.requestDescription(),
        ...location,
      });
      this.requestDescription.set('');
      this.requestFormOpen.set(false);
      this.selectedRequest.set(created);
      this.showToast('Tu solicitud ya está visible en el mapa.');
      await this.loadRequests();
      this.selectRequest(this.mapRequests().find(item => item.id === created.id) ?? created);
    } catch (error) {
      this.requestError.set(this.errorMessage(error, 'No se pudo publicar la solicitud. Inténtalo de nuevo.'));
    } finally {
      this.savingRequest.set(false);
    }
  }

  async joinRequest(request: MapRequest) {
    this.requestError.set('');
    try {
      await this.api.joinRequest(request.id);
      await this.loadRequests();
      this.showToast('Te has apuntado a la solicitud.');
    } catch (error) {
      this.requestError.set(this.errorMessage(error, 'No se pudo completar la inscripción.'));
    }
  }

  selectRequest(request: MapRequest) {
    this.selectedRequest.set(request);
    this.map?.panTo([request.lat, request.lng], { animate: true });
    this.renderMarkers();
  }

  private fitRequests(requests: MapRequest[]) {
    if (!this.map || !requests.length) return;
    if (requests.length === 1) {
      this.map.setView([requests[0].lat, requests[0].lng], 13);
      return;
    }
    this.map.fitBounds(
      L.latLngBounds(requests.map(request => [request.lat, request.lng] as L.LatLngTuple)),
      { padding: [50, 50], maxZoom: 14 },
    );
  }

  private renderMarkers() {
    if (!this.map) return;
    this.markers.forEach(marker => marker.remove());
    this.markers = this.mapRequests().map(request => {
      const icon = L.divIcon({
        className: 'request-marker',
        html: `<span class="request-marker-pin ${this.selectedRequest()?.id === request.id ? 'is-selected' : ''}">♡</span>`,
        iconSize: [42, 48],
        iconAnchor: [21, 44],
      });
      const marker = L.marker([request.lat, request.lng], { icon }).addTo(this.map!);
      marker.bindTooltip(this.escapeAttribute(request.creatorName), { direction: 'top', offset: [0, -35] });
      marker.on('click', () => this.selectRequest(request));
      return marker;
    });
    this.renderLocationMarker();
  }

  private renderLocationMarker() {
    if (!this.map) return;
    this.locationMarker?.remove();
    const point = this.requestLocation();
    if (!point) return;
    this.locationMarker = L.circleMarker([point.lat, point.lng], {
      radius: 9, color: '#fff', weight: 3, fillColor: '#6b2b83', fillOpacity: 1,
    }).addTo(this.map);
    this.locationMarker.bindTooltip('Ubicación aproximada de tu solicitud');
  }

  private escapeAttribute(value: string) {
    return value.replace(/[&"<>']/g, character => ({
      '&': '&amp;', '"': '&quot;', '<': '&lt;', '>': '&gt;', "'": '&#39;',
    }[character] ?? character));
  }

  showToast(message: string) {
    this.toast.set(message);
    if (this.toastTimer) clearTimeout(this.toastTimer);
    this.toastTimer = setTimeout(() => this.toast.set(''), 4000);
  }

  private errorMessage(error: unknown, fallback: string) {
    if (error instanceof ApiError) {
      if (error.status === 409) {
        return error.message.includes('solicitud') ? error.message : 'Ya existe una cuenta con ese correo.';
      }
      if (error.status === 401) return 'That email and password do not match.';
      if (error.status === 400) return error.message;
      if (error.status >= 500) return 'The server could not complete your request. Please try again later.';
    }
    return fallback;
  }
}

bootstrapApplication(AppComponent).catch(error => console.error(error));
