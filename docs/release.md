# REL — Build và phát hành Windows

**CC-REL-001 · 0.5 · 2026-11-19 · Chưa nghiệm thu.** Tham chiếu [ISO/IEC/IEEE 15289:2019](https://www.iso.org/standard/74909.html). Phân biệt demo mock, MVP nội bộ và phát hành công khai theo [SRS](requirements.md). Không phát hành mock như bản truyền thật.

## Kiểm và đóng gói

1. Chốt Git SHA và version theo [CMP](configuration-management.md). Ghi phiên bản thực tế của Go, Node/npm và Wails CLI theo lockfile và pin. Nếu chọn helper, dùng toolchain tương thích `go.mod` của engine đã duyệt.
2. Chạy `npm --prefix frontend ci`, `npm --prefix frontend test`, `npm --prefix frontend run build`, rồi `go test ./...`, `go vet ./...` và `govulncheck ./...`. Kiểm cả helper module nếu có. Chạy `npm --prefix frontend audit`, ghi rủi ro, cách xử lý hoặc lý do chấp thuận. Không tuyên bố đã thiết lập CI nếu mới lưu quy trình.
3. Chạy `wails doctor`, sau đó `wails build -platform windows/amd64`. Wails có thể build lại frontend, vì vậy kiểm gói cuối, không suy từ hash của bundle độc lập. Đưa engine đã xác minh, giấy phép, SBOM (CycloneDX hoặc SPDX) và hướng dẫn vào gói. Không include fixture/log hoặc bí mật.
4. Phát hành công khai dùng installer kiểm runtime. Nếu thiếu, dùng [Evergreen Bootstrapper từ Microsoft](https://learn.microsoft.com/microsoft-edge/webview2/concepts/distribution), không tải từ mirror bất kỳ. Nếu offline, báo hướng dẫn dùng Evergreen Standalone phù hợp thay vì mở UI trắng. Cơ chế installer và chiến lược ký phải qua thử nghiệm trước khi chốt.
5. Ký Authenticode và timestamp EXE/installer do dự án dựng trước phân phối công khai. Với binary nguyên bản của upstream, xác minh chữ ký/provenance nếu có và giữ hash gốc, không mô tả chữ ký của installer là chữ ký Croc. Kiểm SmartScreen với gói tải qua trình duyệt và Windows Defender ở cấu hình mặc định. Nếu nghi false positive, lưu version/hash và gửi báo cáo cho nhà cung cấp, không yêu cầu người dùng tắt bảo vệ. Ghi hash gói sau khi ký.
6. Với MVP nội bộ, hoàn thành cổng engine và các ca mục tiêu áp dụng trên hai máy Windows 11 thử nghiệm. Với phát hành công khai, kiểm cùng hash gói cuối trên Windows 10 và 11 x64 sạch, gồm thiếu WebView2, hành vi firewall, runner native và tuyến relay thực. Không tái dùng bằng chứng từ build khác. Xem [V&V](verification.md).

Nếu Go không ở PATH, ghi BLOCKED, không báo PASS. Nếu gói lỗi hoặc phụ thuộc không an toàn, dừng phân phối và quay về artifact đã nghiệm thu có hash xác minh, nếu có. Không xóa dữ liệu đích của người dùng. Hồ sơ phát hành lưu version, Git SHA, hash, SBOM, tuyến kiểm thử, giới hạn và người chốt.
