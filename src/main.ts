import { bootstrapApplication } from '@angular/platform-browser';
import { Component, AfterViewInit, OnDestroy, signal, computed } from '@angular/core';
import { CommonModule } from '@angular/common';
import * as L from 'leaflet';

type Person = { id: string; name: string; age: number; role: string; interests: string[]; distance: string; lat: number; lng: number; color: string; initials: string; online: boolean; bio: string; looking: string; image: string };
const people: Person[] = [
  {id:'demo-sofia',name:'Sofia',age:27,role:'Designer & weekend explorer',interests:['Coffee','Art','Hiking'],distance:'1.2 km away',lat:28.6401,lng:-106.0784,color:'#e6baa8',initials:'S',online:true,bio:'Usually looking for a new coffee spot, a good conversation, or somewhere with a view. Let’s start with your favorite local place.',looking:'A meaningful connection',image:'https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=500&q=85'},
  {id:'demo-daniel',name:'Daniel',age:30,role:'Architect & dog person',interests:['Dogs','Live music','Food'],distance:'2.4 km away',lat:28.6298,lng:-106.0651,color:'#d9b9a5',initials:'D',online:true,bio:'I believe great dates start with a walk, a playlist, and getting a little lost in a new neighborhood.',looking:'Something real',image:'https://images.unsplash.com/photo-1500648767791-00dcc994a43e?w=500&q=85'},
  {id:'demo-valeria',name:'Valeria',age:26,role:'Book lover & curious mind',interests:['Books','Travel','Coffee'],distance:'3.1 km away',lat:28.6482,lng:-106.1019,color:'#cbaeb3',initials:'V',online:false,bio:'Collecting little stories, big memories, and recommendations for books I will absolutely finish someday.',looking:'A genuine relationship',image:'https://images.unsplash.com/photo-1524504388940-b1c1722653e1?w=500&q=85'},
  {id:'demo-alex',name:'Alex',age:29,role:'Chef in the making',interests:['Cooking','Art','Travel'],distance:'4.6 km away',lat:28.6152,lng:-106.0908,color:'#c8ad95',initials:'A',online:true,bio:'If you have a favorite recipe, I would love to hear the story behind it. Bonus points for spontaneous road trips.',looking:'A meaningful connection',image:'https://images.unsplash.com/photo-1506794778202-cad84cf45f1d?w=500&q=85'},
  {id:'demo-camila',name:'Camila',age:28,role:'Music, mountains, and more',interests:['Hiking','Live music','Dogs'],distance:'5.2 km away',lat:28.6584,lng:-106.0508,color:'#d3b5a2',initials:'C',online:false,bio:'Happiest outdoors or at a tiny concert. Looking to meet someone kind who enjoys both plans and surprises.',looking:'Something real',image:'https://images.unsplash.com/photo-1531123897727-8f129e1688ce?w=500&q=85'},
  {id:'demo-mateo',name:'Mateo',age:31,role:'Photographer & traveler',interests:['Travel','Art','Coffee'],distance:'6.8 km away',lat:28.6052,lng:-106.0528,color:'#b8a99d',initials:'M',online:true,bio:'Always noticing the small details. I would love to explore the city with someone who sees the magic in ordinary days.',looking:'A genuine relationship',image:'https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=500&q=85'}
];

@Component({selector:'app-root',standalone:true,imports:[CommonModule],templateUrl:'./app.component.html',styleUrl:'./app.component.css'})
export class AppComponent implements AfterViewInit, OnDestroy {
  people = signal<Person[]>(people);
  selected = signal<Person>(people[0]);
  filter = signal('Everyone');
  search = signal('');
  activeTab = signal('Discover');
  saved = signal<string[]>([]);
  toast = signal('');
  locationOn = signal(false);
  menuOpen = signal(false);
  filtered = computed(() => this.people().filter(p => (this.filter() === 'Everyone' || p.interests.includes(this.filter())) && (`${p.name} ${p.role} ${p.interests.join(' ')}`).toLowerCase().includes(this.search().toLowerCase())));
  private map?: L.Map;
  private markers: L.Marker[] = [];
  private toastTimer?: ReturnType<typeof setTimeout>;
  private searchTimer?: ReturnType<typeof setTimeout>;
  private requestId = 0;
  ngAfterViewInit() { setTimeout(() => this.initMap(), 0); this.loadProfiles(); }
  private async loadProfiles() {
    const currentRequest = ++this.requestId;
    const params = new URLSearchParams();
    if (this.filter() !== 'Everyone') params.set('interest', this.filter());
    if (this.search().trim()) params.set('search', this.search().trim());
    try {
      const response = await fetch(`/api/profiles?${params.toString()}`);
      if (!response.ok) throw new Error(`API returned ${response.status}`);
      const profiles = await response.json() as Person[];
      if (!Array.isArray(profiles)) throw new Error('Invalid profiles response');
      if (currentRequest !== this.requestId) return;
      this.people.set(profiles);
      if (profiles.length && !profiles.some(p => p.id === this.selected().id)) this.selected.set(profiles[0]);
      this.renderMarkers();
    } catch {
      if (currentRequest !== this.requestId) return;
      this.people.set(people);
      this.showToast('Go API is offline. Showing sample profiles.');
      this.renderMarkers();
    }
  }
  ngOnDestroy() { this.map?.remove(); if(this.toastTimer) clearTimeout(this.toastTimer); if(this.searchTimer) clearTimeout(this.searchTimer); }
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
      const icon = L.divIcon({className:'person-marker',html:`<div class="marker-shell ${this.selected().id===p.id?'is-selected':''}"><img src="${this.escapeAttribute(p.image)}" alt=""><span class="marker-pulse"></span></div>`,iconSize:[62,62],iconAnchor:[31,31]});
      const marker = L.marker([p.lat,p.lng],{icon}).addTo(this.map!);
      marker.on('click',()=>this.select(p));
      return marker;
    });
  }
  private escapeAttribute(value:string) { return value.replace(/[&"<>']/g, c => ({'&':'&amp;','"':'&quot;','<':'&lt;','>':'&gt;',"'":'&#39;'}[c] ?? c)); }
  select(p:Person) { this.selected.set(p); this.renderMarkers(); this.map?.panTo([p.lat,p.lng],{animate:true}); }
  setFilter(value:string) { this.filter.set(value); this.loadProfiles(); }
  setSearch(value:string) { this.search.set(value); if(this.searchTimer) clearTimeout(this.searchTimer); this.searchTimer=setTimeout(()=>this.loadProfiles(),250); }
  toggleSaved(p:Person) { this.saved.update(ids=>ids.includes(p.id)?ids.filter(id=>id!==p.id):[...ids,p.id]); this.showToast(this.saved().includes(p.id)?`${p.name} added to your likes`:`${p.name} removed from likes`); }
  showToast(message:string) { this.toast.set(message); if(this.toastTimer) clearTimeout(this.toastTimer); this.toastTimer=setTimeout(()=>this.toast.set(''),3500); }
  requestMeet(p:Person) { this.showToast(`Sign in to connect with ${p.name}. Messaging is coming soon.`); }
  useLocation() {
    if(!navigator.geolocation) { this.showToast('Location is not supported in this browser.'); return; }
    navigator.geolocation.getCurrentPosition(pos=>{ this.locationOn.set(true); this.map?.setView([pos.coords.latitude,pos.coords.longitude],13); this.showToast('Map centered on your location. Your exact position is not shared.'); },()=>this.showToast('Location access was unavailable. Explore the map instead.'),{enableHighAccuracy:false,timeout:10000});
  }
  openTab(tab:string) { this.activeTab.set(tab); if(tab==='Discover') {setTimeout(()=>this.map?.invalidateSize(),0); return;} if(tab==='Likes') {this.showToast(this.saved().length?`You have ${this.saved().length} saved ${this.saved().length===1?'profile':'profiles'}.`:'No likes yet. Save a profile to see it here.'); return;} this.showToast(`${tab} will be available when accounts are connected.`); }
}

bootstrapApplication(AppComponent).catch(err=>console.error(err));
