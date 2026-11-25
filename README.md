<p align="center">
  <img src="branding/croccoc-mark.png" width="112" alt="CrocCoc logo">
</p>

<h1 align="center">CrocCoc</h1>

<p align="center">Gửi và nhận tệp giữa các máy tính bằng mã kết nối, không cần terminal.</p>

<p align="center">
  <a href="https://github.com/KidouNguyen25/CrocCoc/releases">Tải xuống</a> ·
  <a href="docs/README.md">Tài liệu</a> ·
  <a href="CHANGELOG.md">Changelog</a> ·
  <a href="https://github.com/KidouNguyen25/CrocCoc/issues">Báo lỗi</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Windows-10%20%2F%2011%20x64-0078D4" alt="Windows 10/11 x64">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue" alt="Apache 2.0"></a>
</p>

## Giới thiệu

CrocCoc là ứng dụng desktop mã nguồn mở sử dụng [croc](https://github.com/schollz/croc) để truyền tệp qua mạng. Ứng dụng cung cấp giao diện đồ họa cho cả người gửi và người nhận, không yêu cầu cài Croc CLI riêng khi dùng gói phát hành.

- Gửi nhiều tệp hoặc thư mục bằng mã kết nối.
- Xem thông tin tệp và xác nhận trước khi nhận.
- Theo dõi tiến độ và hủy phiên truyền.
- Mã hóa đầu cuối theo giao thức Croc; không ghi đè tệp đã tồn tại.
- Sử dụng relay công cộng mặc định hoặc relay riêng.

## Tải xuống

**Nền tảng mục tiêu:** Windows 10/11 x64, có Microsoft Edge WebView2 Runtime.

| Gói | Mô tả |
| --- | --- |
| Installer (.exe) | Cài đặt ứng dụng trên Windows. |
| Portable (.zip) | Giải nén toàn bộ gói và mở `CrocCoc.exe`. |

Các gói sẽ được cung cấp tại [GitHub Releases](https://github.com/KidouNguyen25/CrocCoc/releases).

> [!NOTE]
> Phiên bản 0.1.0 đang chuẩn bị phát hành. Bản ứng viên chưa ký Authenticode có thể gặp cảnh báo SmartScreen. Bản ổn định chỉ được công bố sau khi hoàn tất nghiệm thu và ký số. Xem [chính sách ký số](CODE_SIGNING.md) và [checklist phát hành](docs/release-checklist.md).

## Cách sử dụng

**Gửi tệp:** Chọn tệp hoặc thư mục, bắt đầu gửi và chia sẻ mã kết nối với người nhận qua kênh tin cậy.

**Nhận tệp:** Nhập mã kết nối, chọn thư mục lưu, kiểm tra danh sách tệp và xác nhận nhận.

### Relay và quyền riêng tư

Nội dung tệp được mã hóa giữa hai đầu truyền. Relay vẫn có thể quan sát metadata mạng như địa chỉ IP, thời điểm và lưu lượng. Relay công cộng do bên thứ ba vận hành nên tốc độ và khả năng kết nối có thể thay đổi.

Để cấu hình relay riêng, đặt đồng thời `CROCCOC_RELAY_ADDRESS` và `CROCCOC_RELAY_PASSWORD` trước khi khởi chạy. Ứng dụng từ chối khởi tạo engine nếu chỉ đặt một biến, thay vì tự chuyển về relay công cộng.

Xem [hướng dẫn cấu hình relay và kiểm tra gói](docs/release-readme.txt) và [mô hình đe dọa](docs/threat-model.md).

## Phát triển

CrocCoc sử dụng Go, Wails v2, React và TypeScript. Frontend và backend giao tiếp qua Wails IPC, không cần API server riêng.

### Yêu cầu

- Go 1.26 trở lên.
- Node.js 24 trở lên.
- Wails CLI v2.17.1.
- Microsoft Edge WebView2 Runtime trên Windows.

### Chạy từ mã nguồn

```powershell
git clone https://github.com/KidouNguyen25/CrocCoc.git
cd CrocCoc
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1
wails doctor
wails dev
```

Thêm thư mục `$(go env GOPATH)\bin` vào `PATH` nếu chưa có. Để truyền tệp thật, ứng dụng cần worker Croc v11.6.4 đã ghim SHA-256. Xem [hướng dẫn engine](backend/engine-spike/README.md) và [quy trình phát hành](docs/release.md).

Để xem riêng giao diện mô phỏng trên trình duyệt (không truyền tệp thật):

```powershell
npm --prefix frontend ci
npm --prefix frontend run dev
```

### Kiểm thử và build

```powershell
npm --prefix frontend ci
npm --prefix frontend test
npm --prefix frontend run build
go test ./...
wails build -platform windows/amd64
```

Build frontend trước khi chạy kiểm thử Go toàn ứng dụng để có asset nhúng. Có thể kiểm thử riêng backend bằng `go test ./backend/...`.

Bản build Wails nằm tại `build/bin/CrocCoc.exe`. Để đóng gói đầy đủ host, worker, checksum và installer, sử dụng [quy trình phát hành](docs/release.md) và [release checklist](docs/release-checklist.md).

## Tài liệu

| Tài liệu | Nội dung |
| --- | --- |
| [Danh mục tài liệu](docs/README.md) | Toàn bộ hồ sơ kỹ thuật |
| [Yêu cầu](docs/requirements.md) | Phạm vi và tiêu chí MVP |
| [Kiến trúc](docs/architecture.md) · [API](docs/transfer-contract.md) | Thành phần, giao tiếp và engine |
| [Bảo mật](docs/threat-model.md) | Mô hình đe dọa và giới hạn |
| [Kiểm thử](docs/verification.md) · [Truy vết](docs/traceability.md) | Tiêu chí và bằng chứng xác minh |
| [Cấu hình](docs/configuration-management.md) · [Phát hành](docs/release.md) | Build, phiên bản và đóng gói |

## Đóng góp

Gửi lỗi và đề xuất qua [GitHub Issues](https://github.com/KidouNguyen25/CrocCoc/issues). Khi báo lỗi, ghi rõ phiên bản ứng dụng, phiên bản Windows và các bước tái hiện; không đính kèm mã kết nối, mật khẩu relay hoặc dữ liệu riêng tư.

## Giấy phép

CrocCoc được phát hành theo [Apache License 2.0](LICENSE).

Cảm ơn dự án [croc](https://github.com/schollz/croc) và [Wails](https://github.com/wailsapp/wails). Giấy phép các thành phần bên thứ ba được liệt kê trong [THIRD_PARTY_LICENSES.txt](THIRD_PARTY_LICENSES.txt).
