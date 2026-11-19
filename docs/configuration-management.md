# CMP — Quản lý cấu hình và thay đổi CrocCoc

| Mã | Phiên bản | Ngày | Trạng thái | Mã nguồn đối chiếu |
| --- | --- | --- | --- | --- |
| CC-CM-001 | 0.2 | 2026-11-19 | Dự thảo để duyệt | `610b622` |

Hồ sơ vận dụng [ISO/IEC/IEEE 15289:2019](https://www.iso.org/standard/74909.html) ở mức thông tin vòng đời. Quy trình bên dưới dành cho CrocCoc, không ngụ ý repo đã bật bảo vệ nhánh hay bắt buộc một nền tảng CI.

## 1. Hạng mục cấu hình

| ID | Hạng mục | Khóa định danh và nguồn xác nhận |
| --- | --- | --- |
| CI-01 | Frontend, adapter và tài nguyên | Git SHA, `frontend/package.json`, `frontend/package-lock.json`, `frontend/src/assets/`. |
| CI-02 | Dịch vụ Go | Git SHA, `backend/transfer/`, `go.mod` và `go.sum`. |
| CI-03 | Wails host và cấu hình build | Git SHA, `main.go`, `app.go`, `wails.json`, phiên bản Wails CLI đã dùng. |
| CI-04 | Hợp đồng, yêu cầu, thiết kế và kiểm thử | Git SHA và mã/phiên bản tài liệu trong [danh mục](README.md). |
| CI-05 | Croc CLI cho bản Windows tương lai | Phiên bản mục tiêu `11.6.4`, nguồn phát hành, kiến trúc và SHA-256 thực tế khi thu được artifact. Bản init chưa có binary đi kèm. |
| CI-06 | Gói phát hành và bằng chứng | Git SHA nguồn, phiên bản build, SHA-256 từng artifact, môi trường và ID run. Chưa có bản phát hành được nghiệm thu. |

`610b622` là điểm tham chiếu init đã push. Nó không phải bản phát hành Windows đã duyệt. `.tmp/`, ZIP mockup, build cục bộ, log và bí mật không thuộc baseline nguồn. Các ảnh/ZIP demo chỉ đưa vào hồ sơ bàn giao khi được yêu cầu riêng.

## 2. Kiểm soát thay đổi

1. Ghi ticket, lý do và các ID yêu cầu, API hoặc ca bị ảnh hưởng.
2. Rà biên thay đổi. Frontend và backend cùng xác nhận khi sửa binding, event, kiểu payload hoặc quy tắc trạng thái. Bản init phân vai Toàn cho frontend, Phát cho backend, Minh cho tích hợp/QA và tài liệu. Đây là đầu mối phối hợp, không phải xác nhận review đã xảy ra.
3. Bổ sung kiểm thử và cập nhật tài liệu, RTM trong cùng thay đổi. Nếu cần đổi giao diện không tương thích, mô tả bên gọi bị ảnh hưởng và cách chuyển đổi.
4. Ghi kết quả review và lệnh kiểm tra trong PR hoặc ticket. Dùng commit tiếng Anh theo mẫu `type(scope): detail`.
5. Khi chốt baseline mới, ghi SHA, phiên bản tài liệu, artifact nếu có, bằng chứng và người xác nhận. Giữ lịch sử Git để truy nguồn thay vì sửa lại hồ sơ đã duyệt.

## 3. Hồ sơ trạng thái và kiểm tra nhất quán

| Mục trong hồ sơ | Nội dung cần ghi |
| --- | --- |
| Định danh | ID hạng mục, phiên bản, SHA hoặc SHA-256, link nguồn. |
| Vòng đời | Dự thảo, đang review, đã duyệt, đã thay thế hoặc không áp dụng. |
| Thay đổi | Ticket/PR, người phụ trách, phụ thuộc, yêu cầu và ca ảnh hưởng. |
| Xác minh | Ca đã chạy, môi trường, kết luận, bằng chứng đã lọc và điểm chưa chạy. |
| Phát hành | Gói đã bàn giao, baseline nguồn, hướng dẫn quay lui và người xác nhận. |

Trước khi bàn giao, kiểm tra không có tệp nhạy cảm hoặc artifact tạm trong nguồn. Đối chiếu lockfile, pin Croc, phiên bản app, danh mục tài liệu và SHA-256 artifact theo [thủ tục phát hành](release.md). Nếu không tái lập đúng cấu hình, ghi điểm sai khác và xác minh lại, không tái sử dụng bằng chứng của build khác.
