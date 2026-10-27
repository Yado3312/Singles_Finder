import { bootstrapApplication } from '@angular/platform-browser';
import { Component, AfterViewInit, OnDestroy, inject, signal, computed } from '@angular/core';
import { ProfileApiService, Profile } from './profile-api.service';
import { CommonModule } from '@angular/common';
import * as L from 'leaflet';

@Component({selector:'app-root',standalone:true,imports:[CommonModule],templateUrl:'./app.component.html',styleUrl:'./app.component.css'})
export class AppComponent implements AfterViewInit, OnDestroy {
  private api = inject(ProfileApiService);
  people = signal<Profile[]>([]);
  selected = signal<Profile | null>(null);
  loading = signal(false);
  apiError = signal('');
  filter = signal('Everyone');
  search = signal('');
  activeTab = signal('Discover');
  saved = signal<string[]>([]);
  toast = signal('');
  locationOn = signal(false);
  filtered = computed(() => this.people());
  private map?: L.Map;
  private markers: L.Marker[] = [];
  private toastTimer?: ReturnType<typeof setTimeout>;
  private searchTimer?: ReturnType<typeof setTimeout>;
  private requestId = 0;
  private request?: AbortController;
  ngAfterViewInit() { setTimeout(() => this.initMap(), 0); this.loadProfiles(); }
  async loadProfiles() {
    const currentRequest = ++this.requestId;
    this.request?.abort();
    this.request = new AbortController();
    this.loading.set(true);
    this.apiError.set('');
    try {
      const profiles = await this.api.list({interest:this.filter(), search:this.search()}, this.request.signal);
      if (currentRequest !== this.requestId) return;
      this.people.set(profiles);
      if (!profiles.length) this.selected.set(null);
      else if (!profiles.some(p => p.id === this.selected()?.id)) this.selected.set(profiles[0]);
      this.renderMarkers();
    } catch {
      if (currentRequest !== this.requestId) return;
      this.people.set([]);
      this.selected.set(null);
      this.apiError.set('Could not load profiles. Check that the Go API and MongoDB are running.');
      this.renderMarkers();
    } finally {
      if (currentRequest === this.requestId) this.loading.set(false);
    }
  }
  ngOnDestroy() { this.request?.abort(); this.map?.remove(); if(this.toastTimer) clearTimeout(this.toastTimer); if(this.searchTimer) clearTimeout(this.searchTimer); }
  private initMap() {
    this.map = L.map('map', {zoomControl:false, scrollWheelZoom:false}).setView([28.6345,-106.0810], 13);
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png',{attribution:'&copy; OpenStreetMap contributors',maxZoom:19}).addTo(this.map);
    L.control.zoom({position:'bottomright'}).addTo(this.map);
    this.renderMarkers();
  }
  private renderMarkers() {
    if(!this.map) return;
    this.markers.forEach(m=>m.remove());
    this.markers = this.filtered().map(p => {
      const icon = L.divIcon({className:'person-marker',html:`<div class="marker-shell ${this.selected()?.id===p.id?'is-selected':''}"><img src="${this.escapeAttribute(p.image)}" alt=""><span class="marker-pulse"></span></div>`,iconSize:[62,62],iconAnchor:[31,31]});
      const marker = L.marker([p.lat,p.lng],{icon}).addTo(this.map!);
      marker.on('click',()=>this.select(p));
      return marker;
    });
  }
  private escapeAttribute(value:string) { return value.replace(/[&"<>']/g, c => ({'&':'&amp;','"':'&quot;','<':'&lt;','>':'&gt;',"'":'&#39;'}[c] ?? c)); }
  select(p:Profile) { this.selected.set(p); this.renderMarkers(); this.map?.panTo([p.lat,p.lng],{animate:true}); }
  setFilter(value:string) { this.filter.set(value); this.loadProfiles(); }
  setSearch(value:string) { this.search.set(value); if(this.searchTimer) clearTimeout(this.searchTimer); this.searchTimer=setTimeout(()=>this.loadProfiles(),250); }
  toggleSaved(p:Profile) { this.saved.update(ids=>ids.includes(p.id)?ids.filter(id=>id!==p.id):[...ids,p.id]); this.showToast(this.saved().includes(p.id)?`${p.name} added to your likes`:`${p.name} removed from likes`); }
  showToast(message:string) { this.toast.set(message); if(this.toastTimer) clearTimeout(this.toastTimer); this.toastTimer=setTimeout(()=>this.toast.set(''),3500); }
  requestMeet(p:Profile) { this.showToast(`Sign in to connect with ${p.name}. Messaging is coming soon.`); }
  useLocation() {
    if(!navigator.geolocation) { this.showToast('Location is not supported in this browser.'); return; }
    navigator.geolocation.getCurrentPosition(pos=>{ this.locationOn.set(true); this.map?.setView([pos.coords.latitude,pos.coords.longitude],13); this.showToast('Map centered on your location. Your exact position is not shared.'); },()=>this.showToast('Location access was unavailable. Explore the map instead.'),{enableHighAccuracy:false,timeout:10000});
  }
  openTab(tab:string) { this.activeTab.set(tab); if(tab==='Discover') {setTimeout(()=>this.map?.invalidateSize(),0); return;} if(tab==='Likes') {this.showToast(this.saved().length?`You have ${this.saved().length} saved ${this.saved().length===1?'profile':'profiles'}.`:'No likes yet. Save a profile to see it here.'); return;} this.showToast(`${tab} will be available when accounts are connected.`); }
}

bootstrapApplication(AppComponent).catch(err=>console.error(err));
