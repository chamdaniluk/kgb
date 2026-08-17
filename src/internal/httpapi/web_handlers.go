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
<title>{{.Title}} · SI CENDIKIA</title><link rel="stylesheet" href="/static/style.css"></head>
<body><header class="topbar"><a class="brand" href="/">SI CENDIKIA <small>KGB ASN</small></a><nav><a href="/panduan">Tatacara</a><a href="/alur">Alur Pengajuan</a>{{if .Authed}}<a class="button button-small" href="/app">Dasbor</a>{{else}}<a class="button button-small" href="/login">Masuk</a>{{end}}</nav></header>
<main class="container">{{.Content}}</main><footer class="footer">Dinas Pendidikan Kabupaten Grobogan · SI CENDIKIA</footer></body></html>`))
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
		_, _ = w.Write([]byte(styleCSS))
	case "/static/app.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(appJS))
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Beranda", `<section class="hero"><div><p class="eyebrow">Dinas Pendidikan Kabupaten Grobogan</p><h1>Kenaikan Gaji Berkala ASN yang cepat dan jelas.</h1><p class="lead">SI CENDIKIA membantu guru mengajukan, memantau, memverifikasi, dan menerbitkan surat KGB secara digital.</p><div class="actions"><a class="button" href="/login">Masuk ke layanan</a><a class="button button-ghost" href="/alur">Lihat alur</a></div></div><div class="hero-card"><strong>Informasi SI CENDIKIA</strong><div id="public-stats" class="stats"><div><b>—</b><span>Guru ASN</span></div><div><b>—</b><span>Berproses</span></div><div><b>—</b><span>Surat terbit</span></div></div><p id="stats-note" class="muted">Memuat statistik agregat…</p></div></section><section class="grid grid-3"><article class="card"><h2>Untuk Guru</h2><p>Login dengan NIP, unggah satu PDF maksimal 5MB, dan pantau status sampai surat terbit.</p></article><article class="card"><h2>Verifikasi berjenjang</h2><p>Pengajuan diperiksa unit kerja, Dinas, lalu ditandatangani pimpinan.</p></article><article class="card"><h2>Jejak audit</h2><p>Setiap submit, keputusan, penerbitan, dan unduhan tercatat.</p></article></section><script>fetch('/api/v1/public/stats').then(r=>r.json()).then(x=>{const d=x.data||{};const b=document.querySelectorAll('#public-stats b');[d.teachers_total,d.submissions_active,d.letters_issued].forEach((v,i)=>b[i].textContent=Number(v||0).toLocaleString('id-ID'));document.querySelector('#stats-note').textContent='Data agregat diperbarui berkala.'}).catch(()=>document.querySelector('#stats-note').textContent='Statistik belum tersedia.')</script>`)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if s.requestAuthenticated(r) {
		http.Redirect(w, r, "/app", http.StatusSeeOther)
		return
	}
	s.renderPage(w, r, "Masuk", `<section class="narrow"><div class="card login-card"><p class="eyebrow">Akses layanan</p><h1>Masuk ke SI CENDIKIA</h1><p class="muted">Satu form untuk guru dan petugas. Guru menggunakan NIP sebagai username dan password.</p><form id="login-form"><label>Username<input name="username" autocomplete="username" required></label><label>Password<input name="password" type="password" autocomplete="current-password" required></label><button class="button" type="submit">Masuk</button><p id="login-error" class="error" role="alert"></p></form></div></section><script>
const form=document.querySelector('#login-form');form.addEventListener('submit',async e=>{e.preventDefault();const fd=new FormData(form);const r=await fetch('/api/v1/auth/login',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({username:fd.get('username'),password:fd.get('password')})});const x=await r.json();if(!r.ok){document.querySelector('#login-error').textContent=x.error?.message||'Login gagal';return}sessionStorage.setItem('csrf',x.data.csrf);location.href='/app'});
</script>`)
}

func (s *Server) handleGuidePage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Tatacara Penggunaan", `<section class="narrow"><p class="eyebrow">Panduan</p><h1>Tatacara Penggunaan</h1><div class="card"><h2>Guru ASN</h2><ol><li>Masuk dengan NIP sebagai username dan password.</li><li>Pilih pengajuan KGB baru jika sudah memenuhi masa dua tahun.</li><li>Isi TMT usulan, cek pratinjau gaji, dan unggah satu PDF maksimal 5MB.</li><li>Kirim pengajuan. Setelah dikirim, data terkunci.</li><li>Pantau timeline. Jika dikembalikan, perbaiki dan kirim ulang sesuai jenjang.</li><li>Unduh surat PDF setelah status Surat Terbit.</li></ol></div><div class="card"><h2>Verifikator Unit dan Dinas</h2><p>Buka antrean, periksa data dan PDF, lalu setujui atau tolak dengan catatan. Catatan penolakan wajib jelas.</p></div><div class="card"><h2>Pimpinan</h2><p>Buka antrean TTE, tinjau konsep surat, masukkan passphrase eSign, lalu terbitkan surat.</p></div><div class="card"><h2>Admin</h2><p>Impor master BKN, kelola unit dan akun petugas, perbarui skala gaji, template nomor, dan audit trail.</p></div></section>`)
}

func (s *Server) handleFlowPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Alur Pengajuan", `<section class="narrow"><p class="eyebrow">Alur layanan</p><h1>Alur Pengajuan KGB</h1><div class="flow"><div><b>1</b><span>Login guru</span></div><i>→</i><div><b>2</b><span>Ajukan + PDF</span></div><i>→</i><div><b>3</b><span>Verifikasi unit</span></div><i>→</i><div><b>4</b><span>Verifikasi Dinas</span></div><i>→</i><div><b>5</b><span>TTE pimpinan</span></div><i>→</i><div><b>6</b><span>Surat terbit</span></div></div><div class="card"><h2>Jika dikembalikan</h2><p>Penolakan unit dikirim ulang ke antrean unit. Penolakan Dinas dikirim ulang langsung ke antrean Dinas. Semua keputusan tersimpan dalam timeline.</p></div></section>`)
}

func (s *Server) handleAppPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "Aplikasi", `<section id="app" class="app-shell"><div class="loading">Memuat aplikasi…</div></section><script src="/static/app.js"></script>`)
}

const styleCSS = `:root{font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#172033;background:#f5f7fb;line-height:1.5}*{box-sizing:border-box}body{margin:0}.topbar{height:66px;background:#102a56;color:#fff;display:flex;align-items:center;justify-content:space-between;padding:0 max(20px,calc((100% - 1180px)/2));gap:20px}.brand{color:#fff;text-decoration:none;font-weight:800;letter-spacing:.03em}.brand small{display:block;font-weight:500;font-size:10px;letter-spacing:0}.topbar nav{display:flex;align-items:center;gap:18px}.topbar nav a:not(.button){color:#dce8ff;text-decoration:none;font-size:14px}.container{max-width:1180px;margin:0 auto;padding:34px 20px 70px}.footer{text-align:center;color:#68748a;padding:24px;font-size:13px}.hero{display:grid;grid-template-columns:1.35fr .9fr;gap:30px;align-items:center;padding:44px 0 58px}.eyebrow{color:#2563eb;text-transform:uppercase;font-weight:800;letter-spacing:.08em;font-size:12px}.hero h1,h1{font-size:clamp(30px,5vw,52px);line-height:1.1;margin:10px 0 18px;color:#102a56}.lead{font-size:19px;color:#536176;max-width:660px}.actions{display:flex;gap:12px;margin-top:26px}.button{display:inline-flex;border:0;border-radius:9px;background:#2563eb;color:#fff;padding:12px 18px;font-weight:750;text-decoration:none;cursor:pointer;align-items:center;justify-content:center}.button:hover{background:#1d4ed8}.button-ghost{background:#e8efff;color:#1d4ed8}.button-small{padding:7px 12px;font-size:13px}.button-danger{background:#dc2626}.button-success{background:#16a34a}.hero-card,.card{background:#fff;border:1px solid #e2e8f0;border-radius:14px;padding:22px;box-shadow:0 8px 26px rgba(15,42,86,.05)}.hero-card{padding:26px}.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:20px}.stats div{background:#eef4ff;border-radius:10px;padding:15px 8px;text-align:center}.stats b{display:block;font-size:26px;color:#1d4ed8}.stats span{font-size:12px;color:#536176}.grid{display:grid;gap:18px}.grid-3{grid-template-columns:repeat(3,1fr)}.card h2{margin-top:0;color:#183b72;font-size:19px}.muted{color:#68748a}.narrow{max-width:780px;margin:0 auto}.login-card{max-width:470px;margin:30px auto}label{display:block;font-weight:700;margin:16px 0;color:#334155}input,select,textarea{display:block;width:100%;margin-top:7px;border:1px solid #cbd5e1;border-radius:8px;padding:11px 12px;background:#fff;font:inherit}textarea{min-height:100px;resize:vertical}.error{color:#b91c1c;min-height:22px}.success{color:#15803d}.flow{display:flex;align-items:center;justify-content:center;gap:10px;flex-wrap:wrap;margin:30px 0 40px}.flow div{display:flex;align-items:center;gap:8px;background:#fff;border:1px solid #dbe4f2;border-radius:12px;padding:12px}.flow b{background:#2563eb;color:#fff;border-radius:99px;width:26px;height:26px;text-align:center;padding-top:2px}.flow i{color:#64748b}.app-shell h1{font-size:34px}.app-head{display:flex;justify-content:space-between;align-items:start;gap:16px;margin-bottom:20px}.toolbar{display:flex;gap:8px;flex-wrap:wrap}.table-wrap{overflow:auto}.table{width:100%;border-collapse:collapse}.table th,.table td{padding:11px;border-bottom:1px solid #e5e7eb;text-align:left;white-space:nowrap;font-size:14px}.table th{color:#475569;background:#f8fafc}.badge{display:inline-block;border-radius:99px;padding:4px 9px;font-size:12px;font-weight:750;background:#dbeafe;color:#1d4ed8}.badge.red{background:#fee2e2;color:#b91c1c}.badge.green{background:#dcfce7;color:#15803d}.badge.purple{background:#ede9fe;color:#6d28d9}.panel{margin-bottom:18px}.two-col{display:grid;grid-template-columns:1fr 1fr;gap:18px}.timeline{border-left:3px solid #dbeafe;padding-left:18px}.timeline-item{margin:0 0 18px}.loading{text-align:center;padding:60px;color:#64748b}.alert{padding:12px;border-radius:9px;background:#fff7ed;color:#9a3412;margin:12px 0}.file-label{border:2px dashed #bfdbfe;padding:25px;text-align:center;border-radius:10px}.hidden{display:none}@media(max-width:760px){.topbar{height:auto;padding:14px 16px;align-items:flex-start}.topbar nav{gap:8px;flex-wrap:wrap;justify-content:flex-end}.topbar nav a:not(.button){display:none}.container{padding:25px 14px 50px}.hero,.grid-3,.two-col{grid-template-columns:1fr}.hero{padding-top:25px}.flow{justify-content:flex-start}.flow i{display:none}.app-head{display:block}.stats b{font-size:21px}}.tabs{display:flex;gap:8px;margin-bottom:14px;border-bottom:2px solid #e5e7eb;padding-bottom:8px}.tab{color:#475569;text-decoration:none;padding:6px 14px;border-radius:8px;font-weight:600;font-size:14px}.tab.active{background:#102a56;color:#fff}.filter-row{display:flex;gap:10px;flex-wrap:wrap;align-items:flex-end}.filter-row input,.filter-row select{flex:1 1 200px}.table{width:100%;border-collapse:collapse;font-size:13.5px}.table th,.table td{padding:8px 10px;border-bottom:1px solid #eef1f6;text-align:left;vertical-align:top}.table th{color:#64748b;font-weight:700;font-size:12px;text-transform:uppercase;letter-spacing:.03em}.badge.purple{background:#ede9fe;color:#6d28d9}.badge.red{background:#fee2e2;color:#b91c1c}.badge.green{background:#dcfce7;color:#15803d}`
