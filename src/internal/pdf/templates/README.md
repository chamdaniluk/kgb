# Template SK KGB Dinas Pendidikan Grobogan

Sumber: salinan resmi `Contoh SK Berkala PNS.docx` / `PPPK.docx`
(identik dengan `/var/www/ekgb/app/Templates/SK/`).

Hanya nilai contoh yang diganti placeholder. Kop, margin, logo, tembusan,
Menimbang/Mengingat tidak diubah.

## PNS (`pns.docx`)

`${nama}` `${tempat_lahir}` `${tanggal_lahir}` `${nip}` `${karpeg}`
`${pangkat_jabatan}` `${unit_kerja}` `${gaji_lama}` `${gaji_baru}`
`${previous_sk_official}` `${previous_sk_date}` `${previous_sk_number}`
`${previous_sk_effective_date}` `${previous_mkg_years}` `${previous_mkg_months}`
`${next_mkg}` `${next_mkg_months}` `${grade}` `${effective_date}` `${next_tmt}`
`${nomor_naskah}` `${tanggal_naskah}` `${ttd_pengirim}`
`${signer_name}` `${signer_employee}`

## PPPK (`pppk.docx`)

Field PNS yang relevan, plus:
`${masa_perjanjian_kerja}` `${perpanjangan_perjanjian_kerja}`
`${signer_job}` `${tahun_naskah}`

`${tanggal_naskah}` di PPPK bisa terpecah antar run Word; pengisi
wajib merapikan run terpecah.
