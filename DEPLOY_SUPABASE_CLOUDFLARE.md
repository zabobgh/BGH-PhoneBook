# คู่มือการติดตั้ง BGH PhoneBook ออนไลน์ (Supabase + Cloudflare Pages)

คู่มือนี้จะพาคุณนำระบบสมุดโทรศัพท์ขึ้นระบบออนไลน์ **ฟรี 100% ตลอดชีพ** ไม่ต้องมีเซิร์ฟเวอร์ ไม่ต้องเปิดคอมพิวเตอร์ทิ้งไว้

---

## ขั้นตอนที่ 1: ตั้งค่าฐานข้อมูล Supabase (ประมาณ 2 นาที)

1. เข้าเว็บไซต์ [supabase.com](https://supabase.com) แล้วลงทะเบียนหรือล็อกอิน (ฟรี)
2. กดปุ่ม **"New Project"**:
   - **Name**: `bgh-phonebook`
   - **Database Password**: ตั้งรหัสผ่านที่ต้องการ (จำไว้ใช้งาน)
   - **Region**: เลือก **Singapore** (จะเร็วที่สุดสำหรับผู้ใช้งานในไทย)
   - กด **Create new project** แล้วรอระบบสร้างประมาณ 1-2 นาที
3. ไปที่เมนู **SQL Editor** (ไอคอน `>_` บนแถบด้านซ้าย):
   - กด **New Query**
   - เปิดไฟล์ [`supabase_schema.sql`](./supabase_schema.sql) ในโปรเจกต์นี้ คัดลอกโค้ดทั้งหมดมาวาง
   - กดปุ่ม **Run** สีเขียว
   - *(ผลลัพธ์: ตาราง `entries` จะถูกสร้างขึ้น พร้อมระบบ RLS และนำเข้าข้อมูลตั้งต้น 268 รายการทันที)*
4. คัดลอก API Keys:
   - ไปที่ **Project Settings** (ไอคอนฟันเฟืองล่างซ้าย) -> **API**
   - คัดลอกค่า **Project URL** (เช่น `https://xyzcompany.supabase.co`)
   - คัดลอกค่า **Project API Keys** -> `anon` / `public`
5. สร้างบัญชีผู้ดูแลระบบ (Admin User):
   - ไปที่เมนู **Authentication** -> **Users**
   - กดปุ่ม **Add User** -> **Create User**
   - ใส่ Email (เช่น `admin@bgh.go.th`) และรหัสผ่านที่ต้องการ
   - *(บัญชีนี้จะใช้สำหรับกดล็อกอินปุ่ม "เข้าสู่ระบบเจ้าหน้าที่" เพื่อเพิ่ม/แก้/ลบ/ย้ายข้อมูล)*

---

## ขั้นตอนที่ 2: ทดสอบบนเครื่องตัวเอง (Local Test)

1. ในโฟลเดอร์ `frontend/` ให้สร้างไฟล์ชื่อ `.env` โดยคัดลอกจาก `.env.example`:
   ```env
   VITE_SUPABASE_URL=https://your-project-id.supabase.co
   VITE_SUPABASE_ANON_KEY=eyJh......your-anon-key
   ```
2. เปิด Terminal ในโฟลเดอร์ `frontend` แล้วสั่งรัน:
   ```bash
   npm run dev
   ```
3. เปิดเบราว์เซอร์ไปที่ `http://localhost:5173`:
   - ผู้ใช้ทั่วไปจะเห็นโหมดค้นหาเบอร์และคัดลอกเบอร์
   - คลิกปุ่ม **"เข้าสู่ระบบเจ้าหน้าที่"** ที่มุมขวาบน แล้วใส่ Email/Password ที่สร้างในขั้นตอนที่ 1 เพื่อทดสอบปลดล็อกฟังก์ชัน Admin

---

## ขั้นตอนที่ 3: เอาขึ้น Cloudflare Pages (ฟรี ไม่จำกัด Bandwidth)

มีให้เลือก 2 วิธีที่สะดวก:

### วิธีที่ A: อัปโหลดโฟลเดอร์ dist ตรงๆ (ง่ายและเร็วที่สุด ไม่ต้องใช้ Git)
1. สั่ง Build หน้าเว็บ:
   ```bash
   cd frontend
   npm run build
   ```
   *(คุณจะได้โฟลเดอร์ `frontend/dist`)*
2. เข้าเว็บ [dash.cloudflare.com](https://dash.cloudflare.com) ล็อกอิน (ฟรี)
3. เมนูด้านซ้ายเลือก **Workers & Pages** -> กด **Create application**
4. เลือกแท็บ **Pages** -> เลือก **Upload assets**
5. ตั้งชื่อโปรเจกต์ เช่น `bgh-phonebook`
6. ลากโฟลเดอร์ `frontend/dist` ไปวางในกรอบอัปโหลด
7. กด **Deploy site**
8. เรียบร้อย! คุณจะได้ URL ใช้งานทันที เช่น `https://bgh-phonebook.pages.dev`

---

### วิธีที่ B: เชื่อมต่อผ่าน GitHub (อัปเดตโค้ดอัตโนมัติเมื่อ push)
1. Push โค้ดโปรเจกต์นี้ขึ้น GitHub Repository ของคุณ
2. ใน Cloudflare Pages เลือก **Connect to Git** แล้วเลือก Repo ของคุณ
3. ตั้งค่า Build:
   - **Framework preset**: `Vite`
   - **Root directory**: `frontend`
   - **Build command**: `npm run build`
   - **Build output directory**: `dist`
4. ในส่วน **Environment variables (advanced)** ให้เพิ่ม 2 ค่า:
   - Variable name: `VITE_SUPABASE_URL` -> Value: ค่า URL ของ Supabase
   - Variable name: `VITE_SUPABASE_ANON_KEY` -> Value: ค่า anon key ของ Supabase
5. กด **Save and Deploy**
6. เว็บจะ Build และออนไลน์ให้ทันที พร้อมอัปเดตอัตโนมัติทุกครั้งที่แก้โค้ด

---

## สรุปฟังก์ชันในเวอร์ชันออนไลน์

- 🌐 **ผู้ใช้ทั่วไป (Guest)**: ค้นหาเบอร์แบบ Super Omni-Search ได้เร็วทันที, คัดลอกเบอร์, กรองตึก/ชั้น, Export Excel, สั่งพิมพ์ (ไม่เห็นปุ่มแก้/ลบ/เพิ่ม)
- 🔐 **เจ้าหน้าที่ (Admin)**: กดปุ่ม "เข้าสู่ระบบเจ้าหน้าที่" ที่มุมขวาบน เพื่อปลดล็อกปุ่ม เพิ่มหน่วยงาน, แก้ไข, ลบ, ย้ายอาคาร/ชั้น และนำเข้า Excel
- ⚡ **ความเร็ว**: หน้าเว็บโหลดผ่าน Cloudflare CDN เซิร์ฟเวอร์ในกรุงเทพฯ รวดเร็วและเสถียร
