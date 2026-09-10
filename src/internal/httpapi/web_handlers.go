package httpapi

import (
	_ "embed"
	"html/template"
	"net/http"
	"os"
	"time"

	"sicendikia/internal/store"
)

//go:embed assets/logo-grobogan.png
var logoGroboganPNG []byte

//go:embed assets/fonts/plus-jakarta-sans-var.woff2
var fontPJSVar []byte

const assetVersion = "20260909i"

type pageData struct {
	Title   string
	Content template.HTML
	Authed  bool
}

func loadTemplates() *template.Template {
	head := `<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="SI CENDIKIA — layanan Kenaikan Gaji Berkala ASN Dinas Pendidikan Kabupaten Grobogan">
<title>{{.Title}} · SI CENDIKIA</title><link rel="icon" href="/favicon.ico"><link rel="stylesheet" href="/static/style.css?v=` + assetVersion + `">`
	return template.Must(template.New("page").Parse(`<!doctype html>
<html lang="id"><head>` + head + `</head>
<body><a class="skip-link" href="#main">Langsung ke konten</a><header class="topbar"><div class="topbar-inner"><a class="brand" href="/"><span class="brand-mark"><img src="/static/logo-grobogan.png" alt="Lambang Kabupaten Grobogan"></span><span><b>SI CENDIKIA</b><small>Dinas Pendidikan · Kab. Grobogan</small></span></a><nav class="nav" aria-label="Navigasi publik"><a href="/panduan">Tatacara</a><a href="/alur">Alur Pengajuan</a>{{if .Authed}}<a class="btn btn-small btn-primary" href="/app"><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><rect x="3" y="3" width="7" height="9"/><rect x="14" y="3" width="7" height="5"/><rect x="14" y="12" width="7" height="9"/><rect x="3" y="16" width="7" height="5"/></svg>Dasbor</a>{{else}}<a class="btn btn-small btn-primary" href="/login"><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><path d="M10 17l5-5-5-5"/><path d="M15 12H3"/></svg>Masuk</a>{{end}}</nav></div></header>
<main class="container" id="main">{{.Content}}</main><footer class="footer"><div class="footer-inner"><img src="/static/logo-grobogan.png" alt="" width="28" height="36"><span>Dinas Pendidikan Kabupaten Grobogan · SI CENDIKIA — Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN · 2026</span></div></footer></body></html>`))
}

// loadAppTemplate menghasilkan halaman polos untuk SPA /app (tanpa topbar publik).
func loadAppTemplate() *template.Template {
	head := `<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="Dasbor SI CENDIKIA — layanan Kenaikan Gaji Berkala ASN Dinas Pendidikan Kabupaten Grobogan">
<title>{{.Title}} · SI CENDIKIA</title><link rel="icon" href="/favicon.ico"><link rel="stylesheet" href="/static/style.css?v=` + assetVersion + `">`
	return template.Must(template.New("apppage").Parse(`<!doctype html>
<html lang="id"><head>` + head + `</head><body><a class="skip-link" href="#main">Langsung ke konten</a><main id="main">{{.Content}}</main></body></html>`))
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, title string, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.Templates.ExecuteTemplate(w, "page", pageData{Title: title, Content: template.HTML(content), Authed: s.requestAuthenticated(r)}); err != nil {
		http.Error(w, "halaman gagal dirender", http.StatusInternalServerError)
	}
}

func (s *Server) renderAppPage(w http.ResponseWriter, r *http.Request, title string, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.AppTemplate.ExecuteTemplate(w, "apppage", pageData{Title: title, Content: template.HTML(content)}); err != nil {
		http.Error(w, "halaman gagal dirender", http.StatusInternalServerError)
	}
}

// requestAuthenticated membaca sesi tanpa mengubah sesi atau mengaudit aksi.
// Halaman publik hanya memakai hasil ini untuk mengganti tombol Masuk menjadi
// Dasbor; otorisasi endpoint tetap dilakukan oleh middleware withAuth.
func (s *Server) requestAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" || s.Sessions == nil || s.Pool == nil {
		return false
	}
	userID, _, ok := s.Sessions.Verify(cookie.Value)
	if !ok {
		return false
	}
	user, err := store.GetUserByID(r.Context(), s.Pool, userID)
	return err == nil && user.IsActive
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/static/style.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(styleCSS))
	case "/static/app.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte(appJS))
	case "/static/logo-grobogan.png":
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800")
		_, _ = w.Write(logoGroboganPNG)
	case "/static/fonts/plus-jakarta-sans-var.woff2":
		w.Header().Set("Content-Type", "font/woff2")
		w.Header().Set("Cache-Control", "public, max-age=2592000")
		_, _ = w.Write(fontPJSVar)
	default:
		http.NotFound(w, r)
	}
}

func trialNoticeTarget() time.Time {
	if t, ok := lookupTrialUntilEnv(); ok {
		return t
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).AddDate(0, 0, 7)
}

// lookupTrialUntilEnv membaca env TRIAL_UNTIL (cadangan bila admin belum
// mengatur lewat dasbor); ok=false bila kosong atau berformat salah.
func lookupTrialUntilEnv() (t time.Time, ok bool) {
	v := os.Getenv("TRIAL_UNTIL")
	if v == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local), true
	}
	return time.Time{}, false
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	authed := s.requestAuthenticated(r)
	cta := `<a class="btn btn-light" href="/login">Masuk ke layanan<svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 5l7 7-7 7"/></svg></a>`
	if authed {
		cta = `<a class="btn btn-light" href="/app">Buka Dasbor<svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 5l7 7-7 7"/></svg></a>`
	}
	trial := s.trialNotice(r.Context(), r)
	trialUntilStr := trial.Until.Format(time.RFC3339)
	trialUntilTampil := trial.UntilTampil
	trialSection := `<section class="trial-notice reveal" role="status" aria-label="Pengumuman masa uji coba">
<div class="trial-notice-head"><span class="trial-chip"><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 9v4"/><path d="M12 17h.01"/><path d="M10.3 3.9L1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z"/></svg>Pengumuman Dinas</span><span class="trial-badge">Masa Uji Coba Internal</span></div>
<h2>Pemberitahuan Pelaksanaan Uji Coba Internal</h2>
<p>Disampaikan dengan hormat bahwa Sistem Informasi Kenaikan Gaji Berkala Aparatur Sipil Negara (SI CENDIKIA) pada Dinas Pendidikan Kabupaten Grobogan saat ini <b>berada dalam masa uji coba internal</b>. Selama masa uji coba, seluruh data, pengajuan, dan surat yang diterbitkan <b>bersifat sementara dan tidak memiliki kekuatan administrasi maupun hukum</b>, semata-mata untuk keperluan pengujian dan penyempurnaan sistem.</p>
<p>Masa uji coba internal ini akan berakhir pada tanggal <b>` + trialUntilTampil + `</b>. Berkenaan dengan hal tersebut, dimohon kepada seluruh Aparatur Sipil Negara di lingkungan Dinas Pendidikan Kabupaten Grobogan untuk berkenan memberikan masukan, saran, dan melaporkan setiap kendala yang ditemui kepada administrator sistem. Atas perhatian dan kerja sama yang baik, diucapkan terima kasih.</p>
<div class="trial-countdown" data-trial-until="` + trialUntilStr + `" aria-label="Hitung mundur berakhirnya masa uji coba">
<div class="trial-count-head"><span>Sisa Waktu Masa Uji Coba</span><b id="trial-state">Menghitung…</b></div>
<div class="trial-units">
<div class="trial-unit"><b id="trial-days">—</b><span>Hari</span></div>
<div class="trial-unit"><b id="trial-hours">—</b><span>Jam</span></div>
<div class="trial-unit"><b id="trial-mins">—</b><span>Menit</span></div>
<div class="trial-unit"><b id="trial-secs">—</b><span>Detik</span></div>
</div>
<p class="trial-note" id="trial-note">Layanan uji coba tetap dapat digunakan sebagaimana mestinya hingga batas waktu berakhir.</p>
</div>
</section>
`
	if !trial.Enabled {
		trialSection = ``
	}
	s.renderPage(w, r, "Beranda", trialSection+`<section class="hero">
<div class="hero-main reveal">
<span class="hero-chip"><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 21h18"/><path d="M5 21V7l7-4 7 4v14"/><path d="M9 21v-6h6v6"/></svg>Dinas Pendidikan Kabupaten Grobogan</span>
<h1>Kenaikan Gaji Berkala ASN yang <span class="grad-text">cepat dan jelas.</span></h1>
<p class="lead">SI CENDIKIA membantu guru mengajukan, memantau, memverifikasi, dan menerbitkan surat KGB secara digital — dalam satu alur yang jelas, dari ponsel maupun komputer.</p>
<div class="actions">`+cta+`
<a class="btn btn-outline-light" href="/alur">Lihat alur pengajuan</a>
</div>
<div class="hero-trust">
<span><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Satu berkas PDF</span>
<span><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Verifikasi berjenjang</span>
<span><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Tanda tangan elektronik</span>
</div>
</div>
<aside class="hero-card reveal" aria-label="Ringkasan layanan">
<div class="hero-card-head"><span class="hero-card-title">Informasi Layanan</span><span class="hero-card-live"><span class="live-dot" aria-hidden="true"></span>Data langsung</span></div>
<div class="stats" id="public-stats">
<div class="stat"><b id="s-guru">—</b><span>Guru ASN</span></div>
<div class="stat"><b id="s-proses">—</b><span>Sedang berproses</span></div>
<div class="stat"><b id="s-terbit">—</b><span>Surat terbit</span></div>
</div>
<p class="hero-note" id="stats-note">Memuat ringkasan layanan…</p>
<div class="hero-card-foot">
<span class="kicker-light">Cakupan layanan</span>
<p>Melayani Guru ASN pada seluruh Korwil, SMP, dan SKB di lingkungan Dinas Pendidikan Kabupaten Grobogan — dari pengajuan hingga surat terbit dalam satu alur.</p>
</div>
</aside>
</section>

<section class="section grid3">
<article class="card chart-card reveal">
<div class="chart-head"><div><div class="kicker accent-k">Transparansi</div><h3>Pengajuan per bulan</h3><p class="muted2">Perkembangan jumlah pengajuan KGB dari waktu ke waktu.</p></div><span class="badge">6 bulan terakhir</span></div>
	<div class="bars" id="bars-month" role="img" aria-label="Grafik batang pengajuan per bulan">
	<div class="col"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	<div class="col"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	<div class="col"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	<div class="col"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	<div class="col"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	<div class="col last"><b>—</b><i aria-hidden="true" style="height:8px"></i><span>—</span></div>
	</div>
<p class="muted2" style="margin-top:10px" id="bars-note">Memuat data pengajuan…</p>
</article>

<article class="card chart-card reveal">
<div class="chart-head"><div><div class="kicker accent-k">Posisi proses</div><h3>Pengajuan per status</h3><p class="muted2">Sebaran pengajuan pada setiap tahap layanan.</p></div><span class="badge">Ringkasan terkini</span></div>
	<div class="status-bars" id="status-bars" role="img" aria-label="Grafik sebaran status pengajuan"><div class="srow"><span class="muted2">Memuat…</span></div></div>
<p class="muted2" style="margin-top:10px">Biru untuk proses verifikasi, ungu untuk tanda tangan pimpinan, hijau untuk surat terbit, merah untuk berkas yang dikembalikan.</p>
</article>

<article class="card chart-card reveal" style="display:grid;align-content:start">
<div class="kicker accent-k">Cara kerja</div>
<h3>Alur enam langkah yang jelas</h3>
<div class="flow" aria-label="Alur pengajuan">
<span class="step"><b>1</b> Guru masuk</span>
<span class="step"><b>2</b> Ajukan + 1 PDF</span>
<span class="step"><b>3</b> Verifikasi Unit</span>
<span class="step"><b>4</b> Verifikasi Dinas</span>
<span class="step"><b>5</b> TTE Pimpinan</span>
<span class="step step-ok"><b>6</b> Surat Terbit</span>
</div>
<p class="muted2" style="margin-top:10px">Jika dikembalikan Unit → kembali ke Unit. Jika dikembalikan Dinas → langsung ke Dinas. Seluruh keputusan tercatat pada riwayat pengajuan.</p>
<div style="display:flex;gap:8px;margin-top:14px;flex-wrap:wrap">
<a class="btn btn-ghost btn-small" href="/panduan">Baca tatacara penggunaan</a>
<a class="btn btn-ghost btn-small" href="/alur">Lihat alur pengajuan</a>
</div>
</article>
</section>

<section class="section grid3">
<article class="card feature reveal">
<div class="feature-ic blue"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg></div>
<div class="kicker accent-k">Untuk Guru</div>
<h3>Ajukan dari ponsel, pantau sampai terbit</h3>
	<p class="muted">Masuk dengan NIP, unggah satu berkas PDF maksimal 5 MB, lihat perhitungan gaji otomatis, dan pantau status pengajuan yang jelas di setiap tahap.</p>
	<div class="feature-foot"><a class="btn btn-ghost btn-small" href="/login">Masuk sebagai guru</a></div>
</article>
<article class="card feature reveal">
<div class="feature-ic cyan"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div>
<div class="kicker accent-k">Verifikasi berjenjang</div>
<h3>Unit → Dinas → TTE</h3>
	<p class="muted">Antrean petugas dengan pencarian cepat, tampilan data berdampingan dengan berkas PDF, dan catatan pengembalian yang wajib jelas.</p>
	<div class="feature-foot"><a class="btn btn-ghost btn-small" href="/login">Masuk sebagai petugas</a></div>
</article>
<article class="card feature reveal">
<div class="feature-ic green"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg></div>
<div class="kicker accent-k">Jejak audit</div>
<h3>Setiap aksi tercatat</h3>
	<p class="muted">Pengajuan, keputusan, penerbitan, dan unduhan surat tercatat lengkap dengan waktu, pelaksana, dan rincian.</p>
	<div class="feature-foot"><span class="badge">Akuntabel dan dapat ditelusuri</span></div>
</article>
</section>

<section class="section cta" aria-label="Ajakan">
<div>
<h2>Siap mengajukan KGB?</h2>
<p>Isi TMT usulan, unggah berkas PDF, lalu kirim. Setelah dikirim, status pengajuan Anda langsung tampil di dasbor.</p>
</div>
<div style="display:flex;gap:10px;flex-wrap:wrap">
<a class="btn btn-light" href="/login">Masuk sekarang<svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14"/><path d="M13 5l7 7-7 7"/></svg></a>
<a class="btn btn-outline-light" href="/panduan">Baca tatacara penggunaan</a>
</div>
</section>

<script>
(function(){
var elGuru=document.getElementById('s-guru'),elProses=document.getElementById('s-proses'),elTerbit=document.getElementById('s-terbit'),note=document.getElementById('stats-note');
var dash='—';
function setStats(d){
var fmt=function(n){return Number(n||0).toLocaleString('id-ID')};
if(elGuru)elGuru.textContent=(d.teachers_total!=null)?fmt(d.teachers_total):dash;
if(elProses)elProses.textContent=(d.submissions_active!=null)?fmt(d.submissions_active):dash;
if(elTerbit)elTerbit.textContent=(d.letters_issued!=null)?fmt(d.letters_issued):dash;
if(note)note.textContent='Ringkasan layanan diperbarui secara berkala.';
}
function renderBars(perMonth){
var el=document.getElementById('bars-month'),noteB=document.getElementById('bars-note');
if(!el)return;
var map={};(perMonth||[]).forEach(function(m){map[m.month]=Number(m.count||0)});
var months=[],now=new Date();
for(var i=5;i>=0;i--){var d=new Date(now.getFullYear(),now.getMonth()-i,1);months.push({key:d.getFullYear()+'-'+('0'+(d.getMonth()+1)).slice(-2),label:d.toLocaleDateString('id-ID',{month:'short'})});}
	var vals=months.map(function(m){return map[m.key]||0});
	var max=Math.max.apply(null,vals.concat([1]));
	var total6=vals.reduce(function(a,b){return a+b},0);
	el.setAttribute('aria-label','Grafik batang pengajuan per bulan, total '+total6+' pengajuan');
	el.innerHTML=months.map(function(m,idx){
	var h=Math.max(5,Math.round(vals[idx]/max*100));
	var val=vals[idx];
	var empty=val===0?' empty':'';
	return '<div class="col'+(idx===months.length-1?' last':'')+empty+'"><b>'+val.toLocaleString('id-ID')+'</b><i aria-hidden="true" style="height:'+h+'%" title="'+val+' pengajuan pada '+m.label+'"></i><span>'+m.label+'</span></div>';
	}).join('');
if(noteB)noteB.textContent='Total '+vals.reduce(function(a,b){return a+b},0)+' pengajuan dalam enam bulan terakhir.';
}
function renderStatus(perStatus){
var el=document.getElementById('status-bars');
if(!el)return;
var rows=[
['menunggu_unit','Menunggu Unit','#2563EB'],
['menunggu_dinas','Menunggu Dinas','#2563EB'],
['menunggu_tte','Menunggu TTE','#7C3AED'],
['dikembalikan_unit','Dikembalikan Unit','#DC2626'],
['dikembalikan_dinas','Dikembalikan Dinas','#DC2626'],
['terbit','Surat Terbit','#16A34A']
];
var get=function(k){var v=0;Object.keys(perStatus||{}).forEach(function(s){if(s===k)v=Number(perStatus[s]||0)});return v};
	var vals=rows.map(function(r){return get(r[0])});
	var max=Math.max.apply(null,vals.concat([1]));
	var total=vals.reduce(function(a,b){return a+b},0);
	el.setAttribute('aria-label','Sebaran status pengajuan, total '+total+' pengajuan');
	var html=rows.map(function(r,i){
	var w=Math.max(3,Math.round(vals[i]/max*100));
	var pct=total>0?Math.round(vals[i]/total*100):0;
	return '<div class="srow"><span class="srow-top"><span>'+r[1]+'</span><b>'+vals[i].toLocaleString('id-ID')+' · '+pct+'%</b></span><span class="bar" role="presentation"><i style="width:'+w+'%;background:'+r[2]+'"></i></span></div>';
	}).join('');
if(vals.reduce(function(a,b){return a+b},0)===0){html='<div class="srow"><span class="muted2">Belum ada data pengajuan.</span></div>';}
el.innerHTML=html;
}
fetch('/api/v1/public/stats').then(function(r){return r.json()}).then(function(x){
var d=(x&&x.data)||{};
setStats({teachers_total:d.teachers_total!=null?d.teachers_total:d.teachers,submissions_active:d.submissions_active!=null?d.submissions_active:d.active,letters_issued:d.letters_issued!=null?d.letters_issued:d.issued});
renderBars(d.submissions_per_month);
renderStatus(d.per_status);
}).catch(function(){setStats({});if(note)note.textContent='Ringkasan layanan belum tersedia. Silakan muat ulang beberapa saat lagi.';});
(function(){
var box=document.querySelector('.trial-countdown');
if(!box)return;
var target=new Date(box.getAttribute('data-trial-until'));
var elD=document.getElementById('trial-days'),elH=document.getElementById('trial-hours'),elM=document.getElementById('trial-mins'),elS=document.getElementById('trial-secs'),elSt=document.getElementById('trial-state'),elN=document.getElementById('trial-note');
var pad=function(n){return (n<10?'0':'')+n};
function tick(){
var diff=target.getTime()-Date.now();
if(isNaN(diff))return;
if(diff<=0){
if(elD)elD.textContent='00';if(elH)elH.textContent='00';if(elM)elM.textContent='00';if(elS)elS.textContent='00';
if(elSt)elSt.textContent='Masa uji coba telah berakhir';
if(elN)elN.textContent='Terima kasih atas partisipasi Bapak/Ibu dalam masa uji coba internal. Informasi lebih lanjut akan diumumkan kemudian melalui saluran resmi Dinas.';
box.classList.add('trial-done');
return;
}
var s=Math.floor(diff/1000);
var d=Math.floor(s/86400),h=Math.floor(s%86400/3600),m=Math.floor(s%3600/60),ss=s%60;
if(elD)elD.textContent=pad(d);if(elH)elH.textContent=pad(h);if(elM)elM.textContent=pad(m);if(elS)elS.textContent=pad(ss);
if(elSt)elSt.textContent=d>0?('Tersisa '+d+' hari'):('Tersisa '+h+' jam '+m+' menit');
}
tick();setInterval(tick,1000);
})();
})();
</script>`)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.requestAuthenticated(r) {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	s.renderPage(w, r, "Masuk", `<section class="login-wrap-outer"><div class="login-wrap reveal">
<aside class="login-side">
<img class="login-logo" src="/static/logo-grobogan.png" alt="Lambang Kabupaten Grobogan" width="58" height="74">
<b>SI CENDIKIA</b>
<p>Kenaikan Gaji Berkala ASN<br>Dinas Pendidikan Kabupaten Grobogan</p>
<ul class="login-points">
<li><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Satu akun untuk guru dan petugas</li>
<li><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Status pengajuan dipantau real-time</li>
<li><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5"/></svg>Surat KGB dengan tanda tangan elektronik</li>
</ul>
<span class="login-side-foot">Butuh bantuan? Buka halaman <a href="/panduan">Tatacara Penggunaan</a>.</span>
</aside>
<div class="login-form">
<p class="eyebrow">Akses layanan</p>
<h1>Masuk ke SI CENDIKIA</h1>
<p class="muted">Guru menggunakan NIP sebagai nama pengguna. Petugas menggunakan akun yang diberikan administrator.</p>
<form id="login-form"><label>Nama pengguna<input id="login-username" name="username" autocomplete="username" aria-describedby="login-error" placeholder="NIP atau nama pengguna" required></label><label>Kata sandi<input id="login-password" name="password" type="password" autocomplete="current-password" aria-describedby="login-error" placeholder="Kata sandi Anda" required></label><button class="btn btn-primary btn-block" type="submit" style="margin-top:20px"><svg class="icon icon-sm" viewBox="0 0 24 24" aria-hidden="true"><path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4"/><path d="M10 17l5-5-5-5"/><path d="M15 12H3"/></svg>Masuk</button><p id="login-error" class="error" role="alert"></p></form>
<p class="muted2 login-foot">Sesi berlaku 12 jam. Keluarlah setelah selesai menggunakan komputer bersama.</p>
</div>
</div></section><script>
const form=document.querySelector('#login-form');const uEl=document.querySelector('#login-username'),pEl=document.querySelector('#login-password'),errEl=document.querySelector('#login-error');function setInvalid(on){[uEl,pEl].forEach(el=>{if(on){el.setAttribute('aria-invalid','true')}else{el.removeAttribute('aria-invalid')}})}form.addEventListener('submit',async e=>{e.preventDefault();setInvalid(false);errEl.textContent='';const fd=new FormData(form);const r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:fd.get('username'),password:fd.get('password')})});const x=await r.json();if(!r.ok){errEl.textContent=x.error?.message||'Nama pengguna atau kata sandi tidak sesuai.';setInvalid(true);uEl.focus();return}sessionStorage.setItem('csrf',x.data.csrf);location.href='/app'});
</script>`)
}

func (s *Server) handleGuidePage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Tatacara Penggunaan", `<section class="narrow"><p class="eyebrow">Panduan</p><h1>Tatacara Penggunaan</h1><p class="muted" style="margin-bottom:8px">Panduan ringkas untuk setiap peran pengguna SI CENDIKIA.</p>
<article class="card guide-card"><div class="feature-ic blue"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg></div><div><h2>Guru ASN</h2><ol><li>Masuk dengan NIP sebagai nama pengguna dan kata sandi.</li><li>Ajukan KGB baru jika sudah memenuhi masa dua tahun.</li><li>Isi TMT usulan, periksa perhitungan gaji, dan unggah satu berkas PDF maksimal 5 MB.</li><li>Kirim pengajuan. Setelah dikirim, data tidak dapat diubah.</li><li>Pantau perkembangan pengajuan. Jika dikembalikan, perbaiki lalu kirim ulang sesuai jenjang.</li><li>Unduh surat setelah statusnya Surat Terbit.</li></ol></div></article>
<article class="card guide-card"><div class="feature-ic cyan"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 11l3 3L22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg></div><div><h2>Verifikator Unit dan Dinas</h2><p>Buka antrean, periksa data dan berkas PDF, lalu setujui atau kembalikan disertai catatan. Catatan pengembalian wajib jelas.</p></div></article>
<article class="card guide-card"><div class="feature-ic violet"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="M3 17c3 0 3-10 6-10s3 8 6 8 3-4 6-4"/><path d="M3 21h18"/></svg></div><div><h2>Pimpinan</h2><p>Buka antrean penandatanganan, tinjau konsep surat, lalu terbitkan surat dengan tanda tangan elektronik.</p></div></article>
<article class="card guide-card"><div class="feature-ic amber"><svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 1 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 1 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 1 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 1 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg></div><div><h2>Administrator</h2><p>Kelola data ASN, unit kerja, dan akun petugas; perbarui skala gaji serta penomoran surat; dan telusuri riwayat kegiatan.</p></div></article>
</section>`)
}

func (s *Server) handleFlowPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Alur Pengajuan", `<section class="narrow"><p class="eyebrow">Alur layanan</p><h1>Alur Pengajuan KGB</h1><p class="muted" style="margin-bottom:8px">Enam langkah dari pengajuan hingga surat terbit.</p>
<div class="flow flow-lg" aria-label="Alur pengajuan">
<span class="step"><b>1</b> Guru masuk</span>
<span class="step"><b>2</b> Ajukan + PDF</span>
<span class="step"><b>3</b> Verifikasi Unit</span>
<span class="step"><b>4</b> Verifikasi Dinas</span>
<span class="step"><b>5</b> Tanda tangan Pimpinan</span>
<span class="step step-ok"><b>6</b> Surat terbit</span>
</div>
<article class="card" style="margin-top:20px"><h2>Jika dikembalikan</h2><p>Pengembalian dari Unit dikirim ulang ke antrean Unit. Pengembalian dari Dinas dikirim ulang langsung ke antrean Dinas. Seluruh keputusan tercatat pada riwayat pengajuan.</p></article>
<article class="card"><h2>Aturan berkas</h2><p>Satu berkas PDF, maksimal 5 MB, memuat dokumen pendukung KGB. Catatan pengembalian dari petugas wajib dijadikan acuan perbaikan sebelum mengirim ulang.</p></article>
</section>`)
}

func (s *Server) handleAppPage(w http.ResponseWriter, r *http.Request) {
	s.renderAppPage(w, r, "Aplikasi", `<section id="app" class="app-shell"><div class="loading">Memuat aplikasi…</div></section><script src="/static/app.js?v=`+assetVersion+`"></script>`)
}

const styleCSS = `@font-face{font-family:'Plus Jakarta Sans';font-style:normal;font-weight:400 800;font-display:swap;src:url('/static/fonts/plus-jakarta-sans-var.woff2') format('woff2')}
:root{--bg:#F2F6FB;--card:#FFFFFF;--ink:#13233F;--navy:#13233F;--navy-2:#1E335C;--line:#E8EEF6;--line-2:#DFE8F2;--border:#D8E2EF;--muted:#425573;--muted-2:#64748B;--muted-3:#64748B;--blue:#2563EB;--blue-2:#1D4ED8;--blue-soft:#E8F0FE;--cyan:#0891B2;--cyan-soft:#E0F5FB;--accent:#D97706;--accent-2:#B45309;--accent-soft:#FDF0DD;--violet:#7C3AED;--violet-2:#6D28D9;--violet-soft:#F0EAFF;--success:#16A34A;--success-2:#15803D;--success-soft:#E3F6E9;--danger:#DC2626;--danger-2:#B91C1C;--danger-soft:#FDE9E9;--grad:linear-gradient(135deg,#2563EB 0%,#0EA5E9 60%,#06B6D4 100%);--grad-soft:linear-gradient(135deg,#EAF1FE 0%,#E3F4FD 100%);--radius:18px;--radius-sm:13px;--shadow:0 1px 2px rgba(19,35,63,.05),0 10px 30px -18px rgba(19,35,63,.18);--shadow-2:0 2px 4px rgba(19,35,63,.05),0 18px 44px -20px rgba(19,35,63,.22);--font-sans:'Plus Jakarta Sans',system-ui,-apple-system,'Segoe UI',Roboto,sans-serif;--ring:0 0 0 3px rgba(37,99,235,.22);--space-1:4px;--space-2:8px;--space-3:12px;--space-4:16px;--space-5:24px;--space-6:32px;--space-7:48px;--maxw:1180px;--flow-arrow:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24' fill='none' stroke='%235B7290' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='M5 12h14'/%3E%3Cpath d='M13 5l7 7-7 7'/%3E%3C/svg%3E")}
*{box-sizing:border-box}
	html{scroll-behavior:smooth;background:radial-gradient(1100px 520px at 88% -8%,#DEE9FB 0%,rgba(222,233,251,0) 58%),radial-gradient(900px 480px at -12% 108%,#D8F1F8 0%,rgba(216,241,248,0) 55%),var(--bg)}
	body{margin:0;font-family:var(--font-sans);color:var(--ink);line-height:1.55;min-height:100vh;min-height:100svh;display:flex;flex-direction:column;-webkit-font-smoothing:antialiased;-moz-osx-font-smoothing:grayscale;text-rendering:optimizeLegibility}
h1,h2,h3,h4{font-family:var(--font-sans);letter-spacing:-.02em;color:var(--navy);font-weight:800;line-height:1.2;text-wrap:balance}
a{color:inherit}
.icon{width:18px;height:18px;flex:0 0 auto;stroke:currentColor;fill:none;stroke-width:1.75;stroke-linecap:round;stroke-linejoin:round;vertical-align:middle}
.icon-sm{width:15px;height:15px}
.skip-link{position:absolute;left:8px;top:-60px;z-index:100;background:var(--navy);color:#fff;padding:10px 16px;border-radius:0 0 10px 10px;font-weight:700;text-decoration:none;transition:top .15s}
.skip-link:focus{top:0}
.accent{color:var(--accent)}
.grad-text{background:linear-gradient(92deg,#8BE4FF,#DDF7FF 70%);-webkit-background-clip:text;background-clip:text;color:transparent}
.accent-k{color:var(--accent)}
.topbar{position:sticky;top:0;z-index:30;background:rgba(255,255,255,.86);-webkit-backdrop-filter:blur(12px);backdrop-filter:blur(12px);border-bottom:1px solid var(--line)}
.topbar-inner{max-width:1180px;margin:0 auto;padding:10px 20px;display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px;text-decoration:none;color:var(--navy)}
.brand-mark{height:42px;width:42px;display:grid;place-items:center;background:#fff;border:1px solid var(--line);border-radius:12px;box-shadow:var(--shadow)}
.brand-mark img{height:34px;width:auto;display:block;object-fit:contain}
.brand b{font-size:16px;letter-spacing:.01em;line-height:1;font-weight:800}
.brand small{display:block;font-size:10.5px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted-2);font-weight:700;margin-top:3px}
.nav{display:flex;align-items:center;gap:4px;flex-wrap:wrap}
.nav a{color:var(--muted);text-decoration:none;font-size:13.5px;font-weight:600;padding:8px 13px;border-radius:10px;transition:background .15s,color .15s}
.nav a:hover{background:var(--blue-soft);color:var(--blue-2)}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:1px solid transparent;border-radius:11px;padding:10px 16px;font-weight:700;font-size:14px;cursor:pointer;text-decoration:none;font-family:inherit;transition:background .15s ease,box-shadow .15s ease,border-color .15s ease,color .15s ease,scale .1s ease}
.btn .icon{width:16px;height:16px}
.btn:active{scale:.97}
.btn-primary{background:var(--grad);color:#fff;box-shadow:0 8px 20px -10px rgba(37,99,235,.6)}
.btn-primary:hover{filter:brightness(1.06)}
.btn-light{background:#fff;color:var(--blue-2);box-shadow:0 10px 24px -12px rgba(2,32,71,.5)}
.btn-light:hover{filter:brightness(.97)}
.btn-accent{background:var(--accent);color:#fff}
.btn-accent:hover{background:var(--accent-2)}
.btn-ghost{background:#fff;border-color:var(--border);color:var(--navy);box-shadow:var(--shadow)}
.btn-ghost:hover{border-color:#BACBDF;background:#F6F9FD}
.btn-outline-light{background:rgba(255,255,255,.10);color:#fff;border-color:rgba(255,255,255,.42)}
.btn-outline-light:hover{background:rgba(255,255,255,.18)}
.btn-danger{background:var(--danger);color:#fff}
.btn-danger:hover{background:var(--danger-2)}
.btn-success{background:var(--success);color:#fff}
.btn-success:hover{background:var(--success-2)}
.btn-small{padding:7px 12px;font-size:13px;border-radius:9px}
.btn-block{width:100%}
	.container{position:relative;z-index:1;max-width:var(--maxw);margin:0 auto;padding:var(--space-6) var(--space-5) var(--space-7);width:100%;flex:1 0 auto;display:flex;flex-direction:column}
	.footer{position:relative;z-index:1;color:var(--muted-2);padding:22px 20px 34px;font-size:12.5px;border-top:1px solid var(--line);margin-top:auto;background:rgba(255,255,255,.6)}
.footer-inner{max-width:var(--maxw);margin:0 auto;display:flex;align-items:center;justify-content:center;gap:12px;flex-wrap:wrap}
.footer-inner img{opacity:.9}
	.hero{position:relative;overflow:hidden;border-radius:26px;background:linear-gradient(135deg,#1E3A8A 0%,#2563EB 48%,#0EA5E9 100%);color:#fff;padding:clamp(26px,4vw,46px);display:grid;grid-template-columns:1.16fr .84fr;gap:clamp(20px,3vw,36px);align-items:center;box-shadow:0 24px 60px -28px rgba(30,58,138,.55)}
	.trial-notice{background:#fff;border:1px solid #F0D9A8;border-left:6px solid var(--accent);border-radius:var(--radius-sm);padding:clamp(20px,3vw,30px);box-shadow:var(--shadow);margin-bottom:var(--space-5)}
	.trial-notice h2{margin:12px 0 10px;font-size:clamp(19px,2.6vw,24px)}
	.trial-notice p{color:var(--muted);font-size:14px;max-width:78ch;margin:0 0 10px}
	.trial-notice p:last-of-type{margin-bottom:0}
	.trial-notice-head{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap}
	.trial-chip{display:inline-flex;align-items:center;gap:8px;background:var(--accent-soft);border:1px solid #F0D9A8;color:var(--accent-2);border-radius:999px;padding:6px 13px;font-size:11px;font-weight:800;letter-spacing:.09em;text-transform:uppercase}
	.trial-badge{display:inline-flex;align-items:center;border-radius:8px;padding:5px 11px;font-size:11.5px;font-weight:800;background:var(--navy);color:#fff;letter-spacing:.03em}
	.trial-countdown{margin-top:18px;background:#F8FAFE;border:1px solid var(--line-2);border-radius:14px;padding:16px 18px}
	.trial-count-head{display:flex;align-items:center;justify-content:space-between;gap:10px;flex-wrap:wrap;font-size:12px;font-weight:800;letter-spacing:.07em;text-transform:uppercase;color:var(--muted-2)}
	.trial-count-head b{color:var(--accent-2);font-size:13px;letter-spacing:.02em;text-transform:none}
	.trial-units{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin-top:12px}
	.trial-unit{background:#fff;border:1px solid var(--border);border-radius:12px;padding:12px 8px;text-align:center;box-shadow:var(--shadow)}
	.trial-unit b{display:block;font-size:clamp(24px,3vw,32px);font-weight:800;color:var(--navy);font-variant-numeric:tabular-nums;line-height:1}
	.trial-unit span{display:block;margin-top:6px;font-size:10.5px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--muted-2)}
	.trial-note{margin:12px 0 0;font-size:12.5px;color:var(--muted-2)}
	.trial-countdown.trial-done{border-color:#B7E8C6;background:var(--success-soft)}
	.trial-countdown.trial-done .trial-count-head b{color:var(--success-2)}
	.trial-countdown.trial-done .trial-unit b{color:var(--success-2)}
.hero::before{content:"";position:absolute;inset:0;background:radial-gradient(640px 320px at 88% -20%,rgba(125,211,252,.35),transparent 60%),radial-gradient(520px 300px at -10% 120%,rgba(6,182,212,.25),transparent 55%);pointer-events:none}
.hero::after{content:"";position:absolute;inset:0;background-image:radial-gradient(rgba(255,255,255,.16) 1px,transparent 1px);background-size:24px 24px;-webkit-mask-image:linear-gradient(115deg,transparent 30%,rgba(0,0,0,.9));mask-image:linear-gradient(115deg,transparent 30%,rgba(0,0,0,.9));opacity:.5;pointer-events:none}
.hero>*{position:relative;z-index:1}
.hero-chip{display:inline-flex;align-items:center;gap:8px;background:rgba(255,255,255,.14);border:1px solid rgba(255,255,255,.25);color:#E8F6FF;border-radius:999px;padding:7px 14px;font-size:11.5px;font-weight:700;letter-spacing:.09em;text-transform:uppercase}
.hero h1{color:#fff;font-size:clamp(28px,4.2vw,42px);margin:16px 0 12px;line-height:1.12}
.hero .lead{color:#DCEBFF;font-size:clamp(14.5px,1.6vw,16.5px);max-width:56ch;margin:0;text-wrap:pretty}
.actions{display:flex;gap:var(--space-3);flex-wrap:wrap;margin-top:var(--space-5)}
.hero-trust{display:flex;gap:16px;flex-wrap:wrap;margin-top:20px;color:#CFE4FF;font-size:12.5px;font-weight:600}
.hero-trust span{display:inline-flex;align-items:center;gap:6px}
.hero-trust .icon{width:14px;height:14px;color:#7FE3C4}
.hero-card{background:rgba(255,255,255,.13);-webkit-backdrop-filter:blur(14px);backdrop-filter:blur(14px);border:1px solid rgba(255,255,255,.28);border-radius:20px;padding:20px;box-shadow:0 18px 40px -20px rgba(2,32,71,.55)}
.hero-card-head{display:flex;justify-content:space-between;align-items:center;gap:10px}
.hero-card-title{font-size:13px;font-weight:800;letter-spacing:.02em}
.hero-card-live{display:inline-flex;align-items:center;gap:6px;font-size:10.5px;font-weight:700;letter-spacing:.06em;text-transform:uppercase;color:#C9F5EA}
.live-dot{width:7px;height:7px;border-radius:999px;background:#34E3B0;box-shadow:0 0 0 0 rgba(52,227,176,.6);animation:livepulse 2s infinite}
@keyframes livepulse{0%{box-shadow:0 0 0 0 rgba(52,227,176,.55)}70%{box-shadow:0 0 0 8px rgba(52,227,176,0)}100%{box-shadow:0 0 0 0 rgba(52,227,176,0)}}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:14px}
.stat{background:rgba(255,255,255,.12);border:1px solid rgba(255,255,255,.22);border-radius:14px;padding:13px 10px;text-align:center}
.stat b{font-size:clamp(20px,2.4vw,26px);color:#fff;display:block;line-height:1;font-variant-numeric:tabular-nums;font-weight:800}
.stat span{font-size:10px;color:#CFE4FF;font-weight:700;letter-spacing:.06em;text-transform:uppercase}
.hero-note{margin:10px 2px 0;font-size:11.5px;color:#CFE4FF}
.hero-card-foot{margin-top:14px;border-top:1px solid rgba(255,255,255,.22);padding-top:12px}
.hero-card-foot p{margin:6px 0 0;font-size:12px;color:#DCEBFF;line-height:1.55;text-wrap:pretty}
.kicker-light{font-size:10.5px;font-weight:800;letter-spacing:.09em;text-transform:uppercase;color:#C9F5EA}
.eyebrow{display:inline-flex;align-items:center;gap:8px;font-size:11px;font-weight:800;letter-spacing:.12em;text-transform:uppercase;color:var(--accent)}
.eyebrow::before{content:"";width:18px;height:2px;background:var(--accent);border-radius:999px}
h1{font-size:clamp(28px,4.4vw,42px);margin:12px 0;line-height:1.12}
h2{font-size:clamp(19px,2.6vw,24px);margin:0 0 8px}
.lead{color:var(--muted);font-size:16px;max-width:62ch;margin:0;text-wrap:pretty}
.muted,.muted2{text-wrap:pretty}
.card p,.card li{text-wrap:pretty}
.kicker{font-size:11px;font-weight:800;letter-spacing:.09em;text-transform:uppercase;color:var(--muted-2)}
.muted{color:var(--muted)}
.muted2{color:var(--muted-2);font-size:12.5px}
.section{margin-top:var(--space-6)}
	.grid3{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:var(--space-4);align-items:stretch}
	.card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius-sm);padding:var(--space-5);box-shadow:var(--shadow)}
	.card h3{margin:0 0 6px;font-size:17px;font-weight:800}
	.grid3>.card{height:100%;min-width:0}
	.chart-card{padding:18px;display:flex;flex-direction:column}
	.chart-card .muted2:last-child{margin-bottom:0}
	.chart-head{display:flex;justify-content:space-between;gap:8px 12px;align-items:flex-start;flex-wrap:wrap}
	.chart-head h3{margin:2px 0 4px}
	.chart-head p{margin:0}
	.chart-head .badge{margin-top:2px;flex:0 0 auto}
	.feature{position:relative;display:flex;flex-direction:column}
	.feature-foot{margin-top:auto;padding-top:16px}
.feature-ic{width:44px;height:44px;border-radius:13px;display:grid;place-items:center;margin-bottom:12px}
.feature-ic .icon{width:21px;height:21px}
.feature-ic.blue{background:var(--blue-soft);color:var(--blue-2)}
.feature-ic.cyan{background:var(--cyan-soft);color:var(--cyan)}
.feature-ic.green{background:var(--success-soft);color:var(--success-2)}
.feature-ic.violet{background:var(--violet-soft);color:var(--violet)}
.feature-ic.amber{background:var(--accent-soft);color:var(--accent-2)}
.guide-card{display:flex;gap:16px;align-items:flex-start}
.guide-card .feature-ic{margin-bottom:0;flex:0 0 auto}
.guide-card h2{font-size:18px}
.badge{display:inline-flex;align-items:center;gap:5px;border-radius:8px;padding:4px 9px;font-size:11px;font-weight:700;letter-spacing:.02em;border:1px solid var(--border);background:#F5F8FC;color:var(--muted)}
.badge.red{background:var(--danger-soft);color:var(--danger-2);border-color:#F8C9C9}
.badge.green{background:var(--success-soft);color:var(--success-2);border-color:#B7E8C6}
.badge.purple{background:var(--violet-soft);color:var(--violet);border-color:#DCCDFB}
.badge.blue{background:var(--blue-soft);color:var(--blue-2);border-color:#C4D8FC}
	.bars{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:12px;align-items:end;margin-top:16px;height:176px}
	.bars .col{display:flex;flex-direction:column;gap:8px;justify-content:flex-end;align-items:center;height:100%}
	.bars .col b{font-size:12.5px;font-weight:800;font-variant-numeric:tabular-nums;line-height:1}
	.bars .col i{width:100%;max-width:44px;min-height:8px;background:linear-gradient(180deg,#60A5FA,#2563EB);border-radius:8px 8px 3px 3px;display:block}
	.bars .col.last i{background:linear-gradient(180deg,#34D3EC,#0891B2)}
	.bars .col span{font-size:11px;color:var(--muted-2);font-weight:700}
	.bars .col.empty i{background:#E2E8F0}
	.status-bars{display:grid;gap:10px;margin-top:14px}
	.srow{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:4px 10px;align-items:center;font-size:12.5px}
	.srow .srow-top{display:contents}
	.srow .bar{height:9px;grid-column:1/-1}
	.srow b{font-weight:800;text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
.bar{height:8px;background:#E9EFF7;border-radius:999px;overflow:hidden}
.bar i{display:block;height:100%;background:var(--blue);border-radius:999px}
.flow{display:flex;align-items:center;gap:var(--space-2) var(--space-3);flex-wrap:wrap;margin:var(--space-4) 0 0}
.flow-lg .step{padding:12px 16px;font-size:14px}
.flow .step{display:inline-flex;align-items:center;gap:var(--space-2);background:#fff;border:1px solid var(--border);border-radius:11px;padding:9px 12px;font-size:13px;font-weight:600;box-shadow:var(--shadow)}
.flow .step:not(:last-child)::after{content:"";width:15px;height:15px;margin-inline-start:var(--space-1);flex:0 0 auto;background:currentColor;-webkit-mask:var(--flow-arrow) center/contain no-repeat;mask:var(--flow-arrow) center/contain no-repeat;color:#9FB2CC}
.flow .step b{width:23px;height:23px;border-radius:8px;background:var(--grad);color:#fff;display:grid;place-items:center;font-size:12px;box-shadow:0 6px 14px -6px rgba(37,99,235,.55)}
.flow .step-ok{border-color:#B7E8C6;background:var(--success-soft)}
.flow .step-ok b{background:var(--success);box-shadow:0 6px 14px -6px rgba(22,163,74,.55)}
.flow .arrow{display:none}
.cta{position:relative;overflow:hidden;background:linear-gradient(135deg,#1E3A8A 0%,#2563EB 55%,#0EA5E9 100%);color:#fff;border-radius:22px;padding:clamp(24px,3.4vw,36px) clamp(24px,4vw,44px);display:flex;justify-content:space-between;gap:var(--space-5);align-items:center;flex-wrap:wrap;box-shadow:0 24px 60px -28px rgba(30,58,138,.55)}
.cta::after{content:"";position:absolute;inset:0;background-image:radial-gradient(rgba(255,255,255,.14) 1px,transparent 1px);background-size:22px 22px;-webkit-mask-image:linear-gradient(200deg,rgba(0,0,0,.9),transparent 55%);mask-image:linear-gradient(200deg,rgba(0,0,0,.9),transparent 55%));opacity:.5;pointer-events:none}
.cta>*{position:relative;z-index:1}
.cta h2{color:#fff;margin:0 0 6px}
.cta p{color:#DCEBFF;margin:0;max-width:56ch}
@media (prefers-reduced-motion: no-preference){
.reveal{opacity:0;transform:translateY(10px);animation:reveal .45s ease forwards}
.reveal:nth-child(1){animation-delay:.04s}
.reveal:nth-child(2){animation-delay:.10s}
.reveal:nth-child(3){animation-delay:.16s}
.reveal:nth-child(4){animation-delay:.22s}
}
@keyframes reveal{to{opacity:1;transform:translateY(0)}}
.narrow{max-width:820px;margin:0 auto}
.narrow>.card{margin-top:var(--space-4)}
.narrow>h1+.card,.narrow>.flow+.card{margin-top:var(--space-5)}
.narrow .card p,.narrow .card li{max-width:70ch}
	.login-wrap-outer{display:grid;place-items:center;padding:var(--space-5) 0;flex:1}
.login-wrap{max-width:920px;width:100%;display:grid;grid-template-columns:1fr 1fr;background:#fff;border:1px solid var(--border);border-radius:24px;overflow:hidden;box-shadow:var(--shadow-2)}
.login-side{position:relative;background:linear-gradient(160deg,#1E3A8A 0%,#2563EB 52%,#0EA5E9 100%);color:#fff;padding:38px 36px;display:flex;flex-direction:column;gap:12px}
.login-side::after{content:"";position:absolute;inset:0;background-image:radial-gradient(rgba(255,255,255,.14) 1px,transparent 1px);background-size:22px 22px;-webkit-mask-image:linear-gradient(180deg,transparent 20%,rgba(0,0,0,.85));mask-image:linear-gradient(180deg,transparent 20%,rgba(0,0,0,.85));opacity:.5;pointer-events:none}
.login-side>*{position:relative;z-index:1}
.login-logo{height:66px;width:auto;object-fit:contain;background:rgba(255,255,255,.94);border-radius:14px;padding:7px 9px;box-shadow:0 10px 24px -12px rgba(2,32,71,.6)}
.login-side b{font-size:20px;font-weight:800;letter-spacing:.01em}
.login-side p{margin:0;color:#DCEBFF;font-size:13.5px;line-height:1.6}
.login-points{list-style:none;margin:10px 0 0;padding:0;display:grid;gap:10px}
.login-points li{display:flex;align-items:center;gap:9px;font-size:13px;font-weight:600;color:#EAF4FF}
.login-points .icon{color:#7FE3C4}
.login-side-foot{margin-top:auto;padding-top:18px;font-size:12px;color:#BDD9FA}
.login-side-foot a{color:#fff;font-weight:700}
.login-form{padding:38px 40px}
.login-form h1{font-size:26px;margin:10px 0 8px}
.login-form>.muted{font-size:13.5px;margin-bottom:6px}
.login-foot{margin:16px 0 0}
.login-card{max-width:440px;margin:var(--space-5) auto;padding:var(--space-6)}
.login-logo{height:76px;width:auto;object-fit:contain;margin:0 0 6px}
label{display:block;font-weight:600;margin:var(--space-4) 0 0;color:var(--navy);font-size:13px}
input,select,textarea{display:block;width:100%;margin-top:7px;border:1px solid var(--border);border-radius:11px;padding:10px 13px;background:#fff;font:inherit;font-size:14px;color:var(--ink);transition:border-color .15s,box-shadow .15s}
input:hover,select:hover,textarea:hover{border-color:#BACBDF}
input:focus-visible,select:focus-visible,textarea:focus-visible{outline:none;border-color:var(--blue);box-shadow:var(--ring)}
.btn:focus-visible,a:focus-visible,.pill:focus-visible,.tab:focus-visible,.sb-item:focus-visible{outline:none;box-shadow:var(--ring)}
textarea{min-height:100px;resize:vertical}
input[aria-invalid="true"],select[aria-invalid="true"],textarea[aria-invalid="true"]{border-color:var(--danger)}
input[aria-invalid="true"],select[aria-invalid="true"],textarea[aria-invalid="true"]{border:2px solid var(--danger);background:var(--danger-soft);outline:none}
label.req-missing{color:var(--danger-2)}
.req-note{display:block;margin-top:4px;font-size:12px;font-weight:700;color:var(--danger-2)}
input[aria-invalid="true"]:focus-visible,textarea[aria-invalid="true"]:focus-visible{box-shadow:0 0 0 3px rgba(220,38,38,.22)}
.error{color:var(--danger-2);min-height:22px;font-size:13px;display:flex;align-items:center;gap:6px;margin-top:8px}
.error:empty{display:none}
.success{color:var(--success)}
	.grid{display:grid;gap:var(--space-4)}
	.grid-3{grid-template-columns:repeat(3,minmax(0,1fr));align-items:stretch}
	.grid-3>.stat-card{height:100%;min-width:0}
.app-shell{min-height:100vh}
.app-shell h1{font-size:28px}
.app-shell section{margin-top:var(--space-6)}
.app-shell section>h2{margin-bottom:var(--space-4)}
.appwrap{display:grid;grid-template-columns:266px minmax(0,1fr);min-height:100vh}
.sb{position:sticky;top:0;height:100vh;overflow-y:auto;display:flex;flex-direction:column;gap:6px;background:#fff;border-right:1px solid var(--line);padding:18px 14px;z-index:45}
.sb-brand{display:flex;align-items:center;gap:11px;text-decoration:none;color:var(--navy);padding:6px 8px 16px}
.sb-brand img{height:42px;width:auto;flex:0 0 auto;object-fit:contain;background:#fff;border:1px solid var(--line);border-radius:12px;padding:4px}
.sb-brand b{font-size:15px;font-weight:800;display:block;line-height:1.15}
.sb-brand small{display:block;font-size:10px;letter-spacing:.08em;text-transform:uppercase;color:var(--muted-2);font-weight:700;margin-top:2px}
.sb-label{font-size:10.5px;font-weight:800;letter-spacing:.1em;text-transform:uppercase;color:#93A6C2;margin:10px 10px 2px}
.sb-nav{display:grid;gap:3px}
.sb-item{display:flex;align-items:center;gap:11px;padding:10px 12px;border-radius:12px;color:var(--muted);font-weight:600;font-size:13.5px;text-decoration:none;border:1px solid transparent;transition:background .15s,color .15s,border-color .15s}
.sb-item:hover{background:#F2F6FC;color:var(--ink)}
.sb-item.active{background:var(--grad-soft);color:var(--blue-2);border-color:#D6E4FF;font-weight:700}
.sb-item.active .sb-ic{background:var(--grad);color:#fff;box-shadow:0 8px 16px -8px rgba(37,99,235,.6)}
.sb-ic{width:34px;height:34px;border-radius:10px;background:#F1F5FA;display:grid;place-items:center;color:#5B7290;flex:0 0 auto}
.sb-ic .icon{width:17px;height:17px}
.sb-foot{margin-top:auto;padding-top:14px;border-top:1px solid var(--line);display:grid;gap:12px}
.sb-user{display:flex;align-items:center;gap:10px;padding:0 4px}
.sb-user .avatar{width:38px;height:38px;border-radius:11px;font-size:13px}
.sb-user b{display:block;font-size:13px;line-height:1.2}
.sb-user small{display:block;font-size:11px;color:var(--muted-2);font-weight:600}
.sb-actions{display:grid;grid-template-columns:1fr auto;gap:8px}
.main{min-width:0;display:flex;flex-direction:column}
.main-head{position:sticky;top:0;z-index:35;background:rgba(242,246,251,.88);-webkit-backdrop-filter:blur(12px);backdrop-filter:blur(12px);border-bottom:1px solid var(--line);padding:13px 26px;display:flex;align-items:center;gap:14px}
.main-head h1{font-size:19px;margin:0;line-height:1.2}
.main-head .sub{font-size:12px;color:var(--muted-2);margin-top:2px}
.main-head .grow{flex:1;min-width:0}
.main-head .toolbar{flex:0 0 auto}
.menu-btn{display:none;width:38px;height:38px;border:1px solid var(--border);background:#fff;border-radius:11px;place-items:center;color:var(--navy);cursor:pointer;box-shadow:var(--shadow);flex:0 0 auto}
.sb-backdrop{position:fixed;inset:0;background:rgba(15,30,63,.45);z-index:40;opacity:0;pointer-events:none;transition:opacity .2s}
.sb-backdrop.show{opacity:1;pointer-events:auto}
.main-body{padding:24px 26px 56px;width:100%;max-width:1240px;margin:0 auto}
.app-head{display:flex;justify-content:space-between;align-items:start;gap:var(--space-4);margin-bottom:var(--space-4)}
.app-title{display:flex;align-items:flex-start;gap:var(--space-3)}
.app-logo{height:52px;width:auto;flex:0 0 auto;object-fit:contain;margin-top:2px}
.toolbar{display:flex;gap:8px;flex-wrap:wrap}
.panel .panel{margin-top:var(--space-4)}
.panel>h2:first-child,.panel>h3:first-child{margin-top:0}
	.stat-card{display:grid;grid-template-columns:auto minmax(0,1fr);gap:6px 14px;align-items:start;background:var(--card);border:1px solid var(--border);border-radius:var(--radius-sm);padding:18px;box-shadow:var(--shadow)}
	.stat-ic{width:44px;height:44px;border-radius:13px;display:grid;place-items:center;flex:0 0 auto;grid-row:span 2}
	.stat-ic .icon{width:20px;height:20px}
	.stat-ic.blue{background:var(--blue-soft);color:var(--blue-2)}
	.stat-ic.cyan{background:var(--cyan-soft);color:var(--cyan)}
	.stat-ic.green{background:var(--success-soft);color:var(--success-2)}
	.stat-ic.violet{background:var(--violet-soft);color:var(--violet)}
	.stat-ic.amber{background:var(--accent-soft);color:var(--accent-2)}
	.stat-card .stat-label{font-size:11px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--muted-2)}
	.stat-card .stat-value{display:block;font-size:clamp(18px,2vw,22px);font-weight:800;color:var(--navy);line-height:1.2;font-variant-numeric:tabular-nums;margin-top:3px;overflow-wrap:anywhere}
	.stat-card .stat-sub{display:block;font-size:12px;color:var(--muted-2);margin-top:4px;line-height:1.55}
	.stat-card .stat-body{min-width:0}
	.stat-card .stat-foot{grid-column:2;margin-top:12px}
.stat-card.grad{background:var(--grad);border:0;color:#fff;box-shadow:0 18px 36px -18px rgba(37,99,235,.65)}
.stat-card.grad .stat-label{color:#CFE4FF}
.stat-card.grad .stat-value{color:#fff}
.stat-card.grad .stat-sub{color:#E4F2FF}
.stat-card.grad .stat-ic{background:rgba(255,255,255,.18);color:#fff}
.table-wrap{overflow:auto;border:1px solid var(--border);border-radius:14px;background:#fff;box-shadow:var(--shadow)}
.table{width:100%;border-collapse:collapse;font-size:13px}
.table th,.table td{padding:11px 14px;border-bottom:1px solid var(--line);text-align:left;vertical-align:middle}
.table th{color:var(--muted-2);background:#F5F8FC;font-weight:800;font-size:10.5px;text-transform:uppercase;letter-spacing:.06em;white-space:nowrap;position:sticky;top:0;z-index:1}
.table tbody tr{transition:background .12s}
.table tbody tr:hover{background:#F6F9FD}
.table tr:last-child td{border-bottom:0}
.panel{margin-bottom:var(--space-4);padding:var(--space-5)}
.two-col{display:grid;grid-template-columns:1fr 1fr;gap:var(--space-4) var(--space-5)}
.timeline{border-left:2px solid var(--line-2);padding-left:20px;display:grid;gap:14px}
.timeline-item{margin:0;position:relative}
.timeline-item::before{content:"";position:absolute;left:-26px;top:4px;width:12px;height:12px;border-radius:999px;background:#fff;border:2px solid var(--line-2)}
.timeline-item.done::before{background:var(--success);border-color:var(--success)}
.timeline-item.now::before{background:var(--blue);border-color:var(--blue);box-shadow:0 0 0 4px rgba(37,99,235,.15)}
.loading{text-align:center;padding:60px;color:var(--muted-2)}
.alert{display:flex;align-items:flex-start;gap:8px;padding:12px 14px;border-radius:12px;background:var(--accent-soft);border:1px solid #F6DCAE;color:var(--accent-2);margin:12px 0;font-size:13.5px}
.alert.success{background:var(--success-soft);border-color:#B7E8C6;color:var(--success-2)}
.alert .icon{width:17px;height:17px;margin-top:1px}
.file-label,.details{border:1.5px dashed #C7D6E8;padding:var(--space-4);border-radius:12px;background:#F7FAFD;margin-top:var(--space-4)}
.file-label{padding:26px;text-align:center}
.hidden{display:none}
.tabs{display:inline-flex;gap:3px;margin-bottom:var(--space-4);background:#E9EFF7;border:1px solid var(--line);border-radius:13px;padding:4px;flex-wrap:wrap}
.tab{color:var(--muted);text-decoration:none;padding:8px 15px;border-radius:9px;font-weight:700;font-size:13px;transition:background .15s,color .15s,box-shadow .15s}
.tab:hover{color:var(--navy)}
.tab.active{background:#fff;color:var(--blue-2);box-shadow:var(--shadow)}
.filter-row{display:flex;gap:var(--space-3);flex-wrap:wrap;align-items:flex-end}
.filter-row input,.filter-row select{flex:1 1 200px}
.layout-split{display:grid;grid-template-columns:1.1fr .9fr;gap:var(--space-4);align-items:start}
.stamp{display:inline-flex;align-items:center;gap:5px;border-radius:999px;padding:4px 11px;font-size:11px;font-weight:700;letter-spacing:.02em;border:1px solid #C4D8FC;background:var(--blue-soft);color:var(--blue-2);white-space:nowrap}
.stamp.blue{background:var(--blue-soft);color:var(--blue-2);border-color:#C4D8FC}
.stamp.violet{background:var(--violet-soft);color:var(--violet);border-color:#DCCDFB}
.stamp.green{background:var(--success-soft);color:var(--success-2);border-color:#B7E8C6}
.stamp.red{background:var(--danger-soft);color:var(--danger-2);border-color:#F8C9C9}
.stamp .dot{width:6px;height:6px;border-radius:999px;background:currentColor;flex:0 0 auto}
.pulse .dot{animation:pulse 1.8s infinite}
@keyframes pulse{0%{opacity:1}50%{opacity:.3}100%{opacity:1}}
.avatar{width:32px;height:32px;border-radius:10px;background:var(--blue-soft);color:var(--blue-2);border:1px solid #C4D8FC;display:inline-flex;align-items:center;justify-content:center;font-weight:800;font-size:12px;flex:0 0 auto}
.avatar.o{background:var(--accent-soft);color:var(--accent-2);border-color:#F6DCAE}
.avatar.g{background:var(--success-soft);color:var(--success-2);border-color:#B7E8C6}
.avatar.v{background:var(--violet-soft);color:var(--violet);border-color:#DCCDFB}
.cell-name{display:flex;align-items:center;gap:10px}
.stepper{display:flex;gap:10px;margin:4px 0 16px}
.stepper .step{flex:1;display:flex;align-items:center;gap:9px;background:#fff;border:1px solid var(--border);border-radius:12px;padding:10px 12px;font-weight:600;font-size:13px;color:var(--muted);box-shadow:var(--shadow)}
.stepper .step b{background:var(--grad);color:#fff;border-radius:9px;width:24px;height:24px;display:inline-flex;align-items:center;justify-content:center;font-size:13px;flex:0 0 auto}
.stepper .step.active{border-color:var(--blue);box-shadow:var(--ring);color:var(--ink)}
.stepper .step.done b{background:var(--success)}
.paper{position:relative;overflow:hidden;background:#fff;border:1px solid var(--border);border-radius:14px;box-shadow:var(--shadow-2);padding:26px 30px}
.paper>*{position:relative}
.paper-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;border-bottom:2.5px solid var(--navy);padding-bottom:12px;margin-bottom:16px}
.paper-garuda{width:52px;height:52px;flex:0 0 auto;opacity:.9}
.paper-kop{text-align:center;flex:1}
.watermark{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font-weight:800;font-size:46px;color:var(--navy);opacity:.06;transform:rotate(-8deg);pointer-events:none;letter-spacing:.14em}
.kbd{display:inline-block;background:var(--navy);color:#fff;border-radius:8px;padding:4px 9px;font-size:11px;font-weight:700;font-variant-numeric:tabular-nums}
.mono{font-variant-numeric:tabular-nums}
.search{flex:1 1 220px;display:flex;align-items:center;gap:8px;background:#fff;border:1px solid var(--border);border-radius:11px;padding:8px 12px;transition:border-color .15s,box-shadow .15s;box-shadow:var(--shadow)}
.search:focus-within{border-color:var(--blue);box-shadow:var(--ring)}
.search .icon{color:var(--muted-2)}
.search input{border:0;outline:0;width:100%;font:inherit;font-size:14px;background:transparent;margin:0;padding:0}
.search input:focus-visible{box-shadow:none}
.pill{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--border);background:#fff;border-radius:999px;padding:7px 13px;font-size:12.5px;font-weight:700;cursor:pointer;transition:background .15s,border-color .15s,color .15s}
.pill:hover{border-color:#BACBDF}
.pill.active{background:var(--grad);color:#fff;border-color:transparent;box-shadow:0 8px 18px -10px rgba(37,99,235,.6)}
.queue-head{display:flex;justify-content:space-between;gap:var(--space-3);align-items:center;flex-wrap:wrap;margin-bottom:var(--space-4)}
.queue-tools{display:flex;gap:8px;flex-wrap:wrap;align-items:center;flex:1;justify-content:flex-end}
.table td .cell-name b{display:block;line-height:1.2}
.table .row-act{white-space:nowrap;text-align:right}
.review-side{position:sticky;top:76px;align-self:start}
.pdf-frame{width:100%;height:460px;border:1px solid var(--border);border-radius:12px;background:#F2F6FB}
.checklist{margin:6px 0 0;padding:0;list-style:none;color:var(--muted);font-size:12.5px;display:grid;gap:5px}
.checklist li{display:flex;align-items:flex-start;gap:7px}
.checklist li .icon{width:15px;height:15px;color:var(--success);margin-top:1px}
.tte-steps{display:flex;gap:6px;margin-top:8px;flex-wrap:wrap}
.decision-note{background:var(--success-soft);border:1px solid #B7E8C6;border-radius:12px;padding:12px;margin-top:10px}
.decision-note.plain{background:#F7FAFD;border-color:var(--line-2)}
.pager{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-top:12px;padding:10px 14px;border:1px solid var(--border);border-radius:12px;background:#fff;box-shadow:var(--shadow);font-size:13px}
.pager-size select{width:auto;display:inline-block;margin:0 0 0 6px;padding:6px 10px}
.pager-info{color:var(--muted-2)}
.pager-nav{display:flex;align-items:center;gap:8px}
.pager-page{color:var(--muted-2);font-weight:700;white-space:nowrap}
.btn:disabled{opacity:.45;cursor:not-allowed}
	@media(max-width:1080px){.grid3,.grid-3{grid-template-columns:repeat(2,minmax(0,1fr))}.appwrap{grid-template-columns:1fr}.sb{position:fixed;left:0;top:0;bottom:0;width:282px;height:100vh;transform:translateX(-104%);transition:transform .25s ease;box-shadow:0 0 60px rgba(15,30,63,.25)}.sb.open{transform:none}.menu-btn{display:grid}.main-body{padding:18px 16px 48px}.main-head{padding:11px 16px}}
@media(max-width:1040px){.hero{grid-template-columns:1fr;gap:var(--space-4)}}
@media(max-width:900px){.two-col,.layout-split{grid-template-columns:1fr}.review-side{position:static}}
@media(max-width:880px){.login-wrap{grid-template-columns:1fr;max-width:480px}.login-side{padding:26px 28px;gap:8px}.login-points{display:none}.login-side-foot{margin-top:8px}.login-form{padding:28px 26px}.login-form h1{font-size:23px}}
@media(max-width:720px){.grid3,.grid-3{grid-template-columns:1fr}.trial-units{grid-template-columns:repeat(2,minmax(0,1fr))}.flow{justify-content:flex-start}.flow .arrow{display:none}.app-head{display:block}.app-head .toolbar{margin-top:var(--space-3)}.stepper{flex-direction:column}.topbar-inner{padding:var(--space-2) var(--space-3);flex-wrap:wrap;gap:var(--space-3)}.nav{gap:var(--space-1);justify-content:flex-end}.container{padding:var(--space-5) var(--space-3) var(--space-6)}.bars{min-height:150px}.bars .col{min-height:150px;height:100%}.stat b{font-size:20px}.stat-card .stat-foot{grid-column:1/-1}.table-wrap{overflow:visible;border:0;box-shadow:none;background:transparent;padding:0}.table{display:block;font-size:13.5px}.table thead{display:none}.table tbody{display:grid;gap:12px}.table tbody tr{display:grid;background:#fff;border:1px solid var(--border);border-radius:14px;padding:12px 15px;box-shadow:var(--shadow)}.table tbody tr:last-child td{border-bottom:0}.table td{border:0;padding:7px 0;display:flex;align-items:center;justify-content:flex-end;gap:14px;text-align:right}.table td:not([data-label]){justify-content:flex-start;text-align:left}.table td[data-label]::before{content:attr(data-label);font-size:10px;font-weight:800;letter-spacing:.07em;text-transform:uppercase;color:var(--muted-2);text-align:left;margin-right:auto}.table td .cell-name{justify-content:flex-start;text-align:left}.table .row-act{white-space:normal}.pager{gap:8px}.pager-info{width:100%;order:3;text-align:center}}
@media(hover:none){.table tbody tr:hover{background:transparent}.nav a:hover{background:transparent;color:var(--muted)}.tab:hover{color:var(--muted)}.pill:hover{border-color:var(--border)}.btn-ghost:hover{border-color:var(--border);background:#fff}}
@media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}*{animation:none!important;transition:none!important}}
@media print{body{background:#fff}.topbar,.footer,.skip-link,.toolbar,.main-head,.menu-btn,.sb,.sb-backdrop,.btn,.pager,.tabs,.review-side,.queue-tools{display:none!important}.appwrap{display:block}.main-body{padding:0;max-width:none}.card,.paper,.layout-split{box-shadow:none;border:0;padding:0;margin:0;display:block}.paper{border:1px solid #000}.watermark{display:none}*{color:#000!important}}`
