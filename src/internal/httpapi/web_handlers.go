package httpapi

import (
	"html/template"
	"net/http"

	"sicendikia/internal/store"
)

type pageData struct {
	Title   string
	Content template.HTML
	Authed  bool
}

func loadTemplates() *template.Template {
	return template.Must(template.New("page").Parse(`<!doctype html>
<html lang="id"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="SI CENDIKIA — layanan Kenaikan Gaji Berkala ASN Dinas Pendidikan Kabupaten Grobogan">
<title>{{.Title}} · SI CENDIKIA</title><link rel="preconnect" href="https://fonts.googleapis.com"><link rel="preconnect" href="https://fonts.gstatic.com" crossorigin><link href="https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,700;9..144,800&family=Plus+Jakarta+Sans:wght@400;500;600;700;800&display=swap" rel="stylesheet"><link rel="stylesheet" href="/static/style.css?v=20260820"></head>
<body><a class="skip-link" href="#main">Langsung ke konten</a><header class="topbar"><div class="topbar-inner"><a class="brand" href="/"><span class="brand-mark">SC</span><span><b>SI CENDIKIA</b><small>KGB ASN · Kab. Grobogan</small></span></a><nav class="nav" aria-label="Navigasi publik"><a href="/panduan">Tatacara</a><a href="/alur">Alur Pengajuan</a>{{if .Authed}}<a class="btn btn-small" style="background:#fff;color:var(--navy)" href="/app">Dasbor</a>{{else}}<a class="btn btn-small" style="background:#fff;color:var(--navy)" href="/login">Masuk</a>{{end}}</nav></div></header>
<main class="container" id="main">{{.Content}}</main><footer class="footer">Dinas Pendidikan Kabupaten Grobogan · SI CENDIKIA — Sistem Cepat Efektif Non-stop Digital Informasi Kenaikan Gaji Berkala ASN · 2026</footer></body></html>`))
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, title string, content string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.Templates.ExecuteTemplate(w, "page", pageData{Title: title, Content: template.HTML(content), Authed: s.requestAuthenticated(r)}); err != nil {
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
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Beranda", `<section class="hero">
<div class="hero-main reveal">
<span class="eyebrow">Dinas Pendidikan Kabupaten Grobogan</span>
<h1>Kenaikan Gaji Berkala ASN yang <span class="accent">cepat dan jelas.</span></h1>
<p class="lead">SI CENDIKIA membantu guru mengajukan, memantau, memverifikasi, dan menerbitkan surat KGB secara digital — satu alur, satu aksi jelas per layar.</p>
<div class="actions">
<a class="btn btn-primary" href="/login">Masuk ke layanan →</a>
<a class="btn btn-ghost" href="/alur">Lihat alur</a>
</div>
<p class="muted2" style="margin-top:10px">Tanpa login kamu tetap bisa melihat statistik agregat. Data diperbarui berkala dari <code>GET /api/v1/public/stats</code>.</p>
</div>
<aside class="hero-card reveal" aria-label="Statistik agregat">
<div class="kicker">Informasi SI CENDIKIA</div>
<div class="stats" id="public-stats">
<div class="stat"><b id="s-guru">—</b><span>Guru ASN</span></div>
<div class="stat"><b id="s-proses">—</b><span>Berproses</span></div>
<div class="stat"><b id="s-terbit">—</b><span>Surat terbit</span></div>
</div>
<p class="muted2" id="stats-note" style="margin-top:8px">Memuat statistik agregat…</p>
<div class="mini-bars" aria-label="Sebaran per unit (contoh)">
<div class="kicker" style="margin-top:6px">Sebaran per unit (contoh)</div>
<div class="bar-row"><span>Korwil Grobogan</span><span class="bar" aria-hidden="true"><i style="width:78%"></i></span><span class="muted" style="text-align:right">12</span></div>
<div class="bar-row"><span>SMPN 1 Purwodadi</span><span class="bar" aria-hidden="true"><i style="width:54%"></i></span><span class="muted" style="text-align:right">8</span></div>
<div class="bar-row"><span>SKB Grobogan</span><span class="bar" aria-hidden="true"><i style="width:32%"></i></span><span class="muted" style="text-align:right">4</span></div>
</div>
</aside>
</section>

<section class="section grid3">
<article class="card chart-card reveal">
<div class="chart-head"><div><div class="kicker">Transparansi</div><h3>Pengajuan per bulan</h3><p class="muted2">Grafik batang CSS murni — tanpa library berat, tetap ringan di ponsel.</p></div><span class="badge">6 bulan terakhir</span></div>
<div class="bars" aria-label="Batang per bulan">
<div class="col"><i aria-hidden="true" style="height:28%"></i><span>Mar</span></div>
<div class="col"><i aria-hidden="true" style="height:44%"></i><span>Apr</span></div>
<div class="col"><i aria-hidden="true" style="height:62%"></i><span>Mei</span></div>
<div class="col"><i aria-hidden="true" style="height:38%"></i><span>Jun</span></div>
<div class="col"><i aria-hidden="true" style="height:88%"></i><span>Jul</span></div>
<div class="col"><i aria-hidden="true" style="height:74%;background:linear-gradient(180deg, var(--vermilion), #EA580C)"></i><span>Agu</span></div>
</div>
<p class="muted2" style="margin-top:8px">Total 34 pengajuan · puncak Juli (9)</p>
</article>

<article class="card chart-card reveal">
<div class="chart-head"><div><div class="kicker">Posisi proses</div><h3>Pengajuan per status</h3><p class="muted2">Warna badge konsisten dengan dashboard petugas.</p></div><span class="badge">Realtime agregat</span></div>
<div class="status-bars">
<div class="srow"><span>Menunggu Unit</span><span class="bar" aria-hidden="true"><i style="width:22%;background:var(--blue)"></i></span><b>4</b></div>
<div class="srow"><span>Menunggu Dinas</span><span class="bar" aria-hidden="true"><i style="width:34%;background:var(--blue)"></i></span><b>6</b></div>
<div class="srow"><span>Menunggu TTE</span><span class="bar" aria-hidden="true"><i style="width:12%;background:var(--violet)"></i></span><b>2</b></div>
<div class="srow"><span>Dikembalikan</span><span class="bar" aria-hidden="true"><i style="width:18%;background:var(--danger)"></i></span><b>3</b></div>
<div class="srow"><span>Surat Terbit</span><span class="bar" aria-hidden="true"><i style="width:92%;background:var(--success)"></i></span><b>342</b></div>
</div>
<p class="muted2" style="margin-top:8px">Sama dengan statusLabel di aplikasi: biru proses, ungu TTE, hijau terbit, merah dikembalikan.</p>
</article>

<article class="card chart-card reveal" style="display:grid;align-content:start">
<div class="kicker">Cara kerja</div>
<h3>Alur 6 langkah — tanpa tebak-tebakan</h3>
<div class="flow" aria-label="Alur pengajuan">
<span class="step"><b>1</b> Masuk NIP</span><span class="arrow">→</span>
<span class="step"><b>2</b> Ajukan + 1 PDF</span><span class="arrow">→</span>
<span class="step"><b>3</b> Verifikasi Unit</span><span class="arrow">→</span>
<span class="step"><b>4</b> Verifikasi Dinas</span><span class="arrow">→</span>
<span class="step"><b>5</b> TTE Pimpinan</span><span class="arrow">→</span>
<span class="step step-ok"><b>6</b> Surat Terbit</span>
</div>
<p class="muted2" style="margin-top:10px">Jika ditolak Unit → kembali ke Unit. Jika ditolak Dinas → langsung ke Dinas. Semua keputusan ada di timeline.</p>
<div style="display:flex;gap:8px;margin-top:12px;flex-wrap:wrap">
<a class="btn btn-ghost btn-small" href="/panduan">Baca tatacara</a>
<a class="btn btn-ghost btn-small" href="/alur">Detail alur</a>
</div>
</article>
</section>

<section class="section grid3">
<article class="card">
<div class="kicker" style="color:var(--vermilion)">Untuk Guru</div>
<h3>Ajukan dari ponsel, pantau sampai terbit</h3>
<p class="muted">Login NIP, unggah satu PDF maksimal 5 MB, cek pratinjau gaji otomatis, dan lihat badge stempel status yang tidak ambigu.</p>
<a class="btn btn-ghost btn-small" href="/login" style="margin-top:10px">Masuk sebagai guru</a>
</article>
<article class="card">
<div class="kicker" style="color:var(--blue)">Verifikasi berjenjang</div>
<h3>Unit → Dinas → TTE</h3>
<p class="muted">Antrean petugas dengan filter cepat, split 60/40 (data + PDF sticky), dan catatan penolakan yang wajib jelas.</p>
<a class="btn btn-ghost btn-small" href="/login" style="margin-top:10px">Masuk sebagai petugas</a>
</article>
<article class="card">
<div class="kicker" style="color:var(--success)">Jejak audit</div>
<h3>Setiap aksi tercatat</h3>
<p class="muted">Submit, keputusan, penerbitan, dan unduhan — lengkap dengan waktu, aktor, dan detail untuk audit.</p>
<span class="badge" style="margin-top:10px">Tanpa halaman mati · satu aksi utama per layar</span>
</article>
</section>

<section class="section cta" aria-label="Ajakan">
<div>
<h2>Siap mengajukan KGB?</h2>
<p>Butuh 3 menit: isi TMT usulan, unggah PDF, kirim. Setelah dikirim, status langsung muncul di dasbor guru.</p>
</div>
<div style="display:flex;gap:10px;flex-wrap:wrap">
<a class="btn btn-ghost" href="/login">Masuk sekarang →</a>
<a class="btn btn-outline-light" href="/panduan">Pelajari tatacara</a>
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
if(note)note.textContent='Data agregat diperbarui berkala.';
}
fetch('/api/v1/public/stats').then(function(r){return r.json()}).then(function(x){
var d=(x&&x.data)||{};
setStats({teachers_total:d.teachers_total!=null?d.teachers_total:d.teachers,submissions_active:d.submissions_active!=null?d.submissions_active:d.active,letters_issued:d.letters_issued!=null?d.letters_issued:d.issued});
}).catch(function(){setStats({});if(note)note.textContent='Data statistik belum tersedia — coba muat ulang sebentar lagi.';});
})();
</script>`)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.requestAuthenticated(r) {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	s.renderPage(w, r, "Masuk", `<section class="narrow"><div class="card login-card"><p class="eyebrow">Akses layanan</p><h1>Masuk ke SI CENDIKIA</h1><p class="muted">Satu form untuk guru dan petugas. Guru menggunakan NIP sebagai username dan password.</p><form id="login-form"><label>Username<input name="username" autocomplete="username" required></label><label>Password<input name="password" type="password" autocomplete="current-password" required></label><button class="btn btn-primary" type="submit" style="margin-top:16px;width:100%">Masuk</button><p id="login-error" class="error" role="alert"></p></form></div></section><script>
const form=document.querySelector('#login-form');form.addEventListener('submit',async e=>{e.preventDefault();const fd=new FormData(form);const r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:fd.get('username'),password:fd.get('password')})});const x=await r.json();if(!r.ok){document.querySelector('#login-error').textContent=x.error?.message||'Login gagal';return}sessionStorage.setItem('csrf',x.data.csrf);location.href='/app'});
</script>`)
}

func (s *Server) handleGuidePage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Tatacara Penggunaan", `<section class="narrow"><p class="eyebrow">Panduan</p><h1>Tatacara Penggunaan</h1><div class="card"><h2>Guru ASN</h2><ol><li>Masuk dengan NIP sebagai username dan password.</li><li>Pilih pengajuan KGB baru jika sudah memenuhi masa dua tahun.</li><li>Isi TMT usulan, cek pratinjau gaji, dan unggah satu PDF maksimal 5MB.</li><li>Kirim pengajuan. Setelah dikirim, data terkunci.</li><li>Pantau timeline. Jika dikembalikan, perbaiki dan kirim ulang sesuai jenjang.</li><li>Unduh surat PDF setelah status Surat Terbit.</li></ol></div><div class="card"><h2>Verifikator Unit dan Dinas</h2><p>Buka antrean, periksa data dan PDF, lalu setujui atau tolak dengan catatan. Catatan penolakan wajib jelas.</p></div><div class="card"><h2>Pimpinan</h2><p>Buka antrean TTE, tinjau konsep surat, masukkan passphrase eSign, lalu terbitkan surat.</p></div><div class="card"><h2>Admin</h2><p>Impor master BKN, kelola unit dan akun petugas, perbarui skala gaji, template nomor, dan audit trail.</p></div></section>`)
}

func (s *Server) handleFlowPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Alur Pengajuan", `<section class="narrow"><p class="eyebrow">Alur layanan</p><h1>Alur Pengajuan KGB</h1><div class="flow" aria-label="Alur pengajuan"><span class="step"><b>1</b> Login guru</span><span class="arrow">→</span><span class="step"><b>2</b> Ajukan + PDF</span><span class="arrow">→</span><span class="step"><b>3</b> Verifikasi unit</span><span class="arrow">→</span><span class="step"><b>4</b> Verifikasi Dinas</span><span class="arrow">→</span><span class="step"><b>5</b> TTE pimpinan</span><span class="arrow">→</span><span class="step step-ok"><b>6</b> Surat terbit</span></div><div class="card"><h2>Jika dikembalikan</h2><p>Penolakan unit dikirim ulang ke antrean unit. Penolakan Dinas dikirim ulang langsung ke antrean Dinas. Semua keputusan tersimpan dalam timeline.</p></div></section>`)
}

func (s *Server) handleAppPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Aplikasi", `<section id="app" class="app-shell"><div class="loading">Memuat aplikasi…</div></section><script src="/static/app.js?v=20260820"></script>`)
}

const styleCSS = `:root{--paper:#FFFBF5;--paper-2:#FFF6E8;--ink:#0F2240;--navy:#132E5E;--navy-2:#1E3A5F;--line:#E7DED0;--line-2:#F0E6D3;--muted:#667087;--muted-2:#64748B;--vermilion:#C2410C;--vermilion-2:#9A3412;--blue:#1D4ED8;--blue-soft:#EAF0FF;--violet:#7C3AED;--violet-soft:#F0E9FF;--success:#15803D;--success-2:#166534;--success-soft:#E8F5E9;--danger:#DC2626;--danger-2:#B91C1C;--danger-soft:#FEF2F2;--radius:16px;--radius-sm:12px;--shadow:0 14px 40px rgba(15,34,64,.08);--shadow-2:0 8px 24px rgba(15,34,64,.10);--font-serif:"Fraunces","Georgia",ui-serif,serif;--font-sans:"Plus Jakarta Sans",system-ui,-apple-system,"Segoe UI",sans-serif}
*{box-sizing:border-box}
html{scroll-behavior:smooth}
body{margin:0;font-family:var(--font-sans);color:var(--ink);line-height:1.55;background:radial-gradient(900px 420px at 14% 0%,rgba(194,65,12,.07),transparent 60%),radial-gradient(700px 500px at 96% 12%,rgba(29,78,216,.05),transparent 58%),linear-gradient(180deg,var(--paper) 0%,#FFFEFB 100%);min-height:100vh}
body::before{content:"";position:fixed;inset:0;pointer-events:none;opacity:.20;z-index:0;background-image:url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='160' height='160'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='.9' numOctaves='2' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)' opacity='.08'/%3E%3C/svg%3E");mix-blend-mode:multiply}
h1,h2,h3{font-family:var(--font-serif);letter-spacing:-.02em;color:var(--navy)}
a{color:inherit}
.skip-link{position:absolute;left:8px;top:-60px;z-index:100;background:var(--navy);color:#fff;padding:10px 16px;border-radius:0 0 12px 12px;font-weight:700;text-decoration:none;transition:top .15s}
.skip-link:focus{top:0}
.accent{color:var(--vermilion)}
.topbar{position:sticky;top:0;z-index:30;background:rgba(15,34,64,.97);backdrop-filter:blur(10px);color:#fff;border-bottom:4px solid var(--vermilion)}
.topbar-inner{max-width:1180px;margin:0 auto;padding:12px 20px;display:flex;align-items:center;justify-content:space-between;gap:16px}
.brand{display:flex;align-items:center;gap:12px;text-decoration:none;color:#fff}
.brand-mark{width:38px;height:38px;border-radius:10px;display:grid;place-items:center;background:#fff;color:var(--navy);font-family:var(--font-serif);font-weight:800;letter-spacing:.04em;box-shadow:0 6px 18px rgba(0,0,0,.18)}
.brand b{font-family:var(--font-serif);font-size:17px;letter-spacing:.04em;line-height:1}
.brand small{display:block;font-size:10px;letter-spacing:.12em;text-transform:uppercase;opacity:.80;font-weight:600}
.nav{display:flex;align-items:center;gap:6px;flex-wrap:wrap}
.nav a{color:#DCE8FF;text-decoration:none;font-size:13px;font-weight:600;padding:7px 10px;border-radius:999px}
.nav a:hover{background:rgba(255,255,255,.10)}
.btn{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:0;border-radius:999px;padding:11px 16px;font-weight:800;cursor:pointer;text-decoration:none;transition:transform .14s ease,box-shadow .14s ease,background .14s ease,border-color .14s ease}
.btn:active{transform:translateY(1px)}
.btn-primary{background:var(--vermilion);color:#fff;box-shadow:0 12px 22px rgba(194,65,12,.22)}
.btn-primary:hover{background:var(--vermilion-2)}
.btn-ghost{background:#fff;border:1px solid var(--line);color:var(--navy)}
.btn-ghost:hover{border-color:#D9CFC0;background:var(--paper-2)}
.btn-outline-light{background:transparent;color:#fff;border:1px solid rgba(255,255,255,.22)}
.btn-outline-light:hover{background:rgba(255,255,255,.08)}
.btn-danger{background:var(--danger);color:#fff}
.btn-danger:hover{background:var(--danger-2)}
.btn-success{background:var(--success);color:#fff}
.btn-success:hover{background:var(--success-2)}
.btn-small{padding:8px 12px;font-size:13px}
.container{position:relative;z-index:1;max-width:1180px;margin:0 auto;padding:28px 20px 60px}
.footer{position:relative;z-index:1;text-align:center;color:var(--muted-2);padding:20px 20px 30px;font-size:12.5px}
.hero{display:grid;grid-template-columns:1.28fr .92fr;gap:22px;align-items:start;padding:22px 0 8px}
.hero-main{background:linear-gradient(180deg,#fff 0%,#FFFEFB 100%);border:1px solid var(--line);border-radius:var(--radius);padding:24px 24px 18px;box-shadow:var(--shadow);position:relative;overflow:hidden}
.hero-main::after{content:"";position:absolute;right:-24px;top:-30px;width:180px;height:180px;border-radius:50%;background:radial-gradient(circle at 30% 30%,rgba(194,65,12,.13),transparent 62%);pointer-events:none}
.hero-card{background:#fff;border:1px solid var(--line);border-left:4px solid var(--vermilion);border-radius:var(--radius);padding:18px;box-shadow:var(--shadow)}
.eyebrow{display:inline-flex;align-items:center;gap:8px;font-size:11px;font-weight:800;letter-spacing:.14em;text-transform:uppercase;color:var(--vermilion)}
.eyebrow::before{content:"";width:18px;height:2px;background:var(--vermilion);border-radius:999px}
h1{font-size:clamp(30px,4.8vw,50px);margin:10px 0 12px}
h2{font-size:clamp(20px,2.8vw,28px);margin:0 0 8px}
.lead{color:var(--muted);font-size:16.5px;max-width:62ch;margin:0}
.actions{display:flex;gap:10px;flex-wrap:wrap;margin-top:18px}
.kicker{font-size:11px;font-weight:800;letter-spacing:.10em;text-transform:uppercase;color:var(--muted)}
.muted{color:var(--muted)}
.muted2{color:var(--muted-2);font-size:12.5px}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:12px}
.stat{background:linear-gradient(180deg,#fff 0%,var(--paper-2) 100%);border:1px solid var(--line-2);border-radius:12px;padding:13px 10px;text-align:center}
.stat b{font-family:var(--font-serif);font-size:26px;color:var(--navy);display:block;line-height:1;font-variant-numeric:tabular-nums}
.stat span{font-size:10.5px;color:var(--muted-2);font-weight:800;letter-spacing:.07em;text-transform:uppercase}
.section{margin-top:22px}
.grid3{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}
.card{background:#fff;border:1px solid var(--line);border-radius:var(--radius-sm);padding:16px;box-shadow:0 8px 18px rgba(15,34,64,.04)}
.card h3{margin:0 0 6px;font-size:18px}
.chart-card{padding:16px}
.chart-head{display:flex;justify-content:space-between;gap:12px;align-items:end;flex-wrap:wrap}
.badge{display:inline-flex;align-items:center;border-radius:999px;padding:5px 9px;font-size:11px;font-weight:800;letter-spacing:.06em;text-transform:uppercase;border:1px solid var(--line);background:#FFFEFB;color:var(--muted)}
.badge.red{background:var(--danger-soft);color:var(--danger-2);border-color:#FFD1D1}
.badge.green{background:var(--success-soft);color:var(--success-2);border-color:#BFE9C3}
.badge.purple{background:var(--violet-soft);color:var(--violet);border-color:#DDD0FF}
.bars{display:grid;grid-template-columns:repeat(6,1fr);gap:10px;align-items:end;margin-top:14px;height:120px}
.bars .col{display:grid;gap:6px;justify-items:center;height:100%}
.bars .col i{width:100%;max-width:44px;background:linear-gradient(180deg,var(--navy) 0%,var(--blue) 100%);border-radius:10px 10px 4px 4px;display:block}
.bars .col span{font-size:11px;color:var(--muted);font-weight:700}
.status-bars{display:grid;gap:10px;margin-top:12px}
.srow{display:grid;grid-template-columns:118px 1fr 38px;gap:10px;align-items:center;font-size:12px}
.srow .bar{height:10px}
.srow b{font-weight:800}
.bar{height:8px;background:#F3E9D6;border-radius:999px;overflow:hidden}
.bar i{display:block;height:100%;background:linear-gradient(90deg,var(--navy),var(--blue));border-radius:999px}
.flow{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:12px 0 0}
.flow .step{display:flex;align-items:center;gap:8px;background:#fff;border:1px solid var(--line);border-radius:999px;padding:9px 12px;box-shadow:0 6px 14px rgba(15,34,64,.04);font-size:13px;font-weight:700}
.flow .step b{width:24px;height:24px;border-radius:999px;background:var(--navy);color:#fff;display:grid;place-items:center;font-size:12px}
.flow .step-ok{border-color:var(--success);background:var(--success-soft)}
.flow .step-ok b{background:var(--success)}
.flow .arrow{color:var(--muted-2);font-weight:800}
.cta{background:linear-gradient(180deg,#0F2240 0%,#132E5E 100%);color:#fff;border-radius:var(--radius);padding:22px;display:flex;justify-content:space-between;gap:16px;align-items:center;flex-wrap:wrap;box-shadow:var(--shadow-2);border:1px solid rgba(255,255,255,.10)}
.cta h2{color:#fff;margin:0 0 6px}
.cta p{color:#DCE8FF;margin:0;max-width:56ch}
@media (prefers-reduced-motion: no-preference){
.reveal{opacity:0;transform:translateY(8px);animation:reveal .5s ease forwards}
.reveal:nth-child(1){animation-delay:.04s}
.reveal:nth-child(2){animation-delay:.10s}
.reveal:nth-child(3){animation-delay:.16s}
.reveal:nth-child(4){animation-delay:.22s}
}
@keyframes reveal{to{opacity:1;transform:translateY(0)}}
.narrow{max-width:820px;margin:0 auto}
.login-card{max-width:470px;margin:24px auto}
label{display:block;font-weight:700;margin:14px 0 0;color:var(--navy);font-size:13px}
input,select,textarea{display:block;width:100%;margin-top:7px;border:1px solid var(--line);border-radius:12px;padding:11px 12px;background:#fff;font:inherit;color:var(--ink)}
input:focus,select:focus,textarea:focus{outline:2px solid #C7D7FF;border-color:#9AB0FF}
.btn:focus-visible,a:focus-visible{outline:3px solid #9AB0FF;outline-offset:2px;border-radius:999px}
textarea{min-height:100px;resize:vertical}
.error{color:var(--danger-2);min-height:22px;font-size:13px}
.success{color:var(--success)}
.grid{display:grid;gap:16px}
.grid-3{grid-template-columns:repeat(3,1fr)}
.app-shell h1{font-size:32px}
.app-head{display:flex;justify-content:space-between;align-items:start;gap:16px;margin-bottom:20px}
.toolbar{display:flex;gap:8px;flex-wrap:wrap}
.table-wrap{overflow:auto;border:1px solid var(--line);border-radius:16px;background:#fff;box-shadow:var(--shadow-2)}
.table{width:100%;border-collapse:collapse;font-size:13px}
.table th,.table td{padding:11px 12px;border-bottom:1px solid #F2EBDC;text-align:left;vertical-align:top}
.table th{color:var(--muted-2);background:#FFFEFB;font-weight:700;font-size:11px;text-transform:uppercase;letter-spacing:.03em;white-space:nowrap}
.table tr:last-child td{border-bottom:0}
.panel{margin-bottom:18px;padding:16px}
.two-col{display:grid;grid-template-columns:1fr 1fr;gap:18px}
.timeline{border-left:3px solid #E9E2D2;padding-left:18px;display:grid;gap:14px}
.timeline-item{margin:0;position:relative}
.timeline-item::before{content:"";position:absolute;left:-23px;top:4px;width:12px;height:12px;border-radius:999px;background:#fff;border:2px solid var(--line)}
.timeline-item.done::before{background:var(--success);border-color:var(--success)}
.timeline-item.now::before{background:var(--blue);border-color:var(--blue);box-shadow:0 0 0 6px rgba(29,78,216,.12)}
.loading{text-align:center;padding:60px;color:var(--muted-2)}
.alert{padding:12px;border-radius:12px;background:var(--paper-2);border:1px solid var(--line-2);color:var(--vermilion-2);margin:12px 0}
.alert.success{background:var(--success-soft);border-color:#BFE9C3;color:var(--success-2)}
.file-label{border:1px dashed #E0D6C3;padding:25px;text-align:center;border-radius:12px;background:#FFFEFB}
.hidden{display:none}
.tabs{display:flex;gap:8px;margin-bottom:14px;border-bottom:2px solid var(--line-2);padding-bottom:8px;flex-wrap:wrap}
.tab{color:var(--muted-2);text-decoration:none;padding:6px 14px;border-radius:999px;font-weight:700;font-size:13px}
.tab.active{background:var(--navy);color:#fff}
.filter-row{display:flex;gap:10px;flex-wrap:wrap;align-items:flex-end}
.filter-row input,.filter-row select{flex:1 1 200px}
.layout-split{display:grid;grid-template-columns:1.1fr .9fr;gap:16px}
.stamp{display:inline-block;border-radius:999px;padding:5px 12px;font-size:11px;font-weight:800;text-transform:uppercase;letter-spacing:.02em;border:1.5px solid #C7D7FF;background:var(--blue-soft);color:var(--blue);transform:rotate(-.6deg)}
.stamp.blue{background:var(--blue-soft);color:var(--blue);border-color:#C7D7FF}
.stamp.violet{background:var(--violet-soft);color:var(--violet);border-color:#DDD0FF}
.stamp.green{background:var(--success-soft);color:var(--success-2);border-color:#BFE9C3}
.stamp.red{background:var(--danger-soft);color:var(--danger-2);border-color:#FFD1D1}
.stamp.cap{animation:cap .5s ease}
@keyframes cap{0%{transform:rotate(-8deg) scale(.9);opacity:0}100%{transform:rotate(-.6deg) scale(1);opacity:1}}
.pulse{position:relative}
.pulse::after{content:"";position:absolute;inset:-3px;border-radius:999px;border:1.5px solid currentColor;opacity:0;animation:pulse 1.8s infinite}
@keyframes pulse{0%{transform:scale(.96);opacity:.35}100%{transform:scale(1.12);opacity:0}}
.avatar{width:32px;height:32px;border-radius:999px;background:var(--blue-soft);color:var(--navy);border:1px solid var(--line);display:inline-flex;align-items:center;justify-content:center;font-weight:800;font-size:12px;flex:0 0 auto}
.avatar.o{background:#FFEAD9;color:var(--vermilion-2);border-color:#F6D5BC}
.avatar.g{background:var(--success-soft);color:var(--success-2);border-color:#BFE9C3}
.avatar.v{background:var(--violet-soft);color:var(--violet);border-color:#DDD0FF}
.cell-name{display:flex;align-items:center;gap:9px}
.stepper{display:flex;gap:10px;margin:4px 0 16px}
.stepper .step{flex:1;display:flex;align-items:center;gap:9px;background:#fff;border:1px solid var(--line);border-radius:12px;padding:10px 12px;font-weight:700;font-size:13px;color:var(--muted)}
.stepper .step b{background:var(--navy);color:#fff;border-radius:999px;width:26px;height:26px;display:inline-flex;align-items:center;justify-content:center;font-size:13px;flex:0 0 auto}
.stepper .step.active{border-color:var(--vermilion);box-shadow:0 8px 18px rgba(194,65,12,.14);color:var(--ink)}
.stepper .step.done b{background:var(--success)}
.details{border:1px dashed #E0D6C3;border-radius:12px;background:#FFFEFB;padding:16px}
.paper{position:relative;overflow:hidden;background:linear-gradient(180deg,#fff,#FFFDF8);border:1px solid var(--line);border-radius:16px;box-shadow:var(--shadow);padding:22px 24px}
.paper::before{content:"";position:absolute;inset:0;pointer-events:none;opacity:.35;background:linear-gradient(90deg,transparent 38px,rgba(231,222,208,.55) 38px,rgba(231,222,208,.55) 39px,transparent 39px),repeating-linear-gradient(180deg,transparent 0 28px,rgba(231,222,208,.35) 28px,rgba(231,222,208,.35) 29px)}
.paper>*{position:relative}
.paper-head{display:flex;justify-content:space-between;align-items:flex-start;gap:12px;border-bottom:1px solid var(--line);padding-bottom:12px;margin-bottom:14px}
.watermark{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;font-family:var(--font-serif);font-weight:800;font-size:42px;color:var(--navy);opacity:.07;transform:rotate(-8deg);pointer-events:none;letter-spacing:.1em}
.kbd{display:inline-block;background:#0F2240;color:#fff;border-radius:8px;padding:4px 9px;font-size:11px;font-weight:700;font-variant-numeric:tabular-nums}
.mono{font-variant-numeric:tabular-nums}
.search{flex:1 1 220px;display:flex;align-items:center;gap:8px;background:#fff;border:1px solid var(--line);border-radius:999px;padding:8px 12px}
.search input{border:0;outline:0;width:100%;font:inherit;background:transparent}
.pill{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--line);background:#fff;border-radius:999px;padding:7px 10px;font-size:12px;font-weight:700;cursor:pointer}
.pill.active{background:var(--navy);color:#fff;border-color:var(--navy)}
.queue-head{display:flex;justify-content:space-between;gap:12px;align-items:center;flex-wrap:wrap;margin-bottom:14px}
.queue-tools{display:flex;gap:8px;flex-wrap:wrap;align-items:center;flex:1;justify-content:flex-end}
.table td .cell-name b{display:block;line-height:1.2}
.table .row-act{white-space:nowrap;text-align:right}
.review-side{position:sticky;top:84px;align-self:start}
.pdf-frame{width:100%;height:460px;border:1px solid var(--line);border-radius:12px;background:#FFFEFB;box-shadow:var(--shadow-2)}
.checklist{margin:6px 0 0 18px;padding:0;color:var(--muted-2);font-size:12.5px;display:grid;gap:4px}
.tte-steps{display:flex;gap:6px;margin-top:8px;flex-wrap:wrap}
.decision-note{background:var(--success-soft);border:1px solid #BFE9C3;border-radius:12px;padding:12px;margin-top:10px}
.decision-note.plain{background:#FFFEFB;border-color:var(--line-2)}
.pager{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap;margin-top:12px;padding:10px 12px;border:1px solid var(--line);border-radius:12px;background:#FFFEFB;font-size:13px}
.pager-size select{width:auto;display:inline-block;margin:0 0 0 6px;padding:6px 10px}
.pager-info{color:var(--muted-2)}
.pager-nav{display:flex;align-items:center;gap:8px}
.pager-page{color:var(--muted-2);font-weight:700;white-space:nowrap}
.btn:disabled{opacity:.45;cursor:not-allowed}
@media(max-width:900px){.hero,.grid3,.grid-3,.two-col,.layout-split{grid-template-columns:1fr}.flow{justify-content:flex-start}.flow .arrow{display:none}.app-head{display:block}.stepper{flex-direction:column}.topbar-inner{padding:10px 14px}.container{padding:20px 14px 50px}.bars{height:100px}.srow{grid-template-columns:100px 1fr 32px}}
@media(max-width:760px){.topbar-inner{flex-wrap:wrap;gap:10px}.nav{gap:6px;justify-content:flex-end}.stats b,.stat b{font-size:21px}}
@media(prefers-reduced-motion:reduce){html{scroll-behavior:auto}*{animation:none!important;transition:none!important}}`
