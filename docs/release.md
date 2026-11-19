# REL — Thủ tục build, kiểm chứng và bàn giao CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-REL-001 | 0.2 | 2026-11-19 | Thủ tục dự kiến, chưa nghiệm thu bản phân phối | `610b622` |

Hồ sơ vận dụng [ISO/IEC/IEEE 15289:2019](https://www.iso.org/standard/74909.html) cho thủ tục và hồ sơ bàn giao. Kiến trúc mục tiêu là Windows 10/11 x64. Bản init đã có bundle frontend, chưa có bằng chứng chạy native hay bản phát hành chứa Croc. ZIP mockup trong `.tmp/` không phải bộ cài.

## 1. Chuẩn bị

- Chọn Git SHA nguồn và đọc các giới hạn đã ghi trong [SRS](requirements.md).
- Dùng máy Windows x64 có Go theo `go.mod`, Node theo `frontend/package.json` và WebView2. Phiên bản Wails CLI cho baseline này lấy theo pin `v2.17.1` trong README và Go module. Đối chiếu phiên bản công cụ trên máy trước khi build.
- Cho bản truyền thật, chỉ nhận binary Croc từ nguồn phát hành đã xác nhận. Ghi phiên bản, kiến trúc, nguồn và SHA-256. Cách bố trí binary và installer thuộc CC-07, chưa được triển khai ở init.

## 2. Các bước ở gốc repo

| Bước | Lệnh hoặc tác vụ | Bằng chứng kỳ vọng |
| --- | --- | --- |
| 1. Phụ thuộc frontend | `npm --prefix frontend ci` | Cài theo lockfile, không đổi lockfile khi build. |
| 2. Kiểm thử frontend | `npm --prefix frontend test` | Log số test và kết luận. |
| 3. Bundle frontend | `npm --prefix frontend run build` | Có `frontend/dist/` cho Go nhúng. |
| 4. Kiểm thử Go | `go test ./...` | Kết quả test backend và bridge, cần Go đúng phiên bản. |
| 5. Chuẩn bị desktop | `wails doctor` | Môi trường Windows phù hợp và phụ thuộc có mặt. |
| 6. Build desktop | `wails build -platform windows/amd64` | EXE dự kiến tại `build/bin/CrocCoc.exe`, ghi đường dẫn thực tế từ build log. |
| 7. Kiểm tra gói | Chạy đúng artifact trên cả hai hệ điều hành, tính SHA-256, chạy ca nghiệm thu. | Khớp `CC-T-11` đến `CC-T-16` trong [V&V](verification.md). |

Bước build thành công không thay cho việc mở ứng dụng thật. Một build chỉ có stub phải ghi là bản init/demo, không ghi là đã hỗ trợ truyền tệp thật. Kết quả thực tế ghi theo [mẫu hồ sơ chạy](verification.md), không đánh dấu sẵn.

## 3. Bàn giao

Hồ sơ bàn giao gồm phiên bản, Git SHA, danh sách artifact và SHA-256, Windows/WebView2 đã kiểm, phiên bản Croc, hướng dẫn chạy, giới hạn đã biết và kết quả nghiệm thu. Nếu có installer và binary bên thứ ba, kèm thông tin giấy phép và nguồn tương ứng. Không đưa fixture nhạy cảm hoặc mã kết nối vào artifact hay ảnh chụp.

Chỉ đánh dấu sẵn sàng phát hành bản truyền thật sau khi ca bắt buộc đạt, không còn lỗi chặn gửi/nhận, và người phụ trách đã xem bằng chứng. Tài liệu này không thực hiện công bố, tạo tag hoặc tải artifact lên nơi phát hành.

## 4. Quay lui và lỗi thường gặp

| Tình huống | Xử lý |
| --- | --- |
| Thiếu Go hoặc Wails | Kiểm tra PATH và phiên bản theo README. Ghi `BLOCKED` nếu không thể chuẩn bị môi trường, không suy ra lỗi ứng dụng. |
| Go không tìm thấy bundle | Chạy bước build frontend trước, kiểm tra `frontend/dist/`. |
| Không khởi động được trên Windows | Kiểm tra kiến trúc, WebView2 và build log. Cập nhật bằng chứng smoke theo từng máy. |
| Croc thiếu hoặc sai phiên bản | Không phát hành như bản truyền thật. Xác nhận lại pin, nguồn, artifact và resolver. |
| Phát hiện lỗi chặn sau bàn giao | Dừng phân phối build lỗi. Nếu có bản trước đã nghiệm thu, dùng artifact trước có SHA-256 đã ghi. Nếu chưa từng có, giữ trạng thái chưa phát hành. Giữ nguyên thư mục dữ liệu người dùng. |
