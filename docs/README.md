# Bộ hồ sơ kỹ thuật CrocCoc

**CC-DOC-001 · 0.6 · 2026-11-23 · Đang cập nhật theo tích hợp Windows.** Nguồn init `610b622`; bộ hồ sơ này đã đồng bộ với nhánh tích hợp Windows (`backend/transfer/`, `app.go` 8 binding khớp ICD, `main.go` có `SingleInstanceLock` và `OnBeforeClose`/`OnShutdown`). Xem kết quả chạy thực tế mới nhất tại [verification.md](verification.md).

| Hồ sơ | Nội dung | Khung tham chiếu |
| --- | --- | --- |
| [Yêu cầu](requirements.md) | Phạm vi, MVP và yêu cầu có mã | 29148 |
| [Hợp đồng](transfer-contract.md) | Init so với IPC mục tiêu | 29148, 15289 |
| [Kiến trúc](architecture.md) | Engine, staging, publish, hủy và ADR | 42010 |
| [Mô hình đe dọa](threat-model.md) | Ranh giới, relay, kênh mã và rủi ro | 42010 |
| [Xác minh](verification.md) | Ca và bằng chứng đã chạy | 29119-3 |
| [Truy vết](traceability.md) | Yêu cầu → điểm thực hiện → ca | 29148, 29119-3 |
| [Cấu hình](configuration-management.md) | Baseline, pin, cập nhật và version | 15289 |
| [Phát hành](release.md) | Build, đóng gói, kiểm, bàn giao | 15289 |

Các chuẩn là hướng tổ chức hồ sơ, không phải tuyên bố chứng nhận hay đối chiếu toàn bộ điều khoản. Mốc 25/12/2026 là lịch sử. [Notion](https://kidounguyen.notion.site/CrocCoc-41cf424aba708182998fe301aaed3636) là tham khảo, chưa được đối chiếu về ticket hoặc cam kết.

## Lịch sử hồ sơ

| Revision | Bằng chứng / hồ sơ thay đổi |
| --- | --- |
| 0.2 | `e7e46ad`, bộ hồ sơ IEEE ban đầu, không đổi mã init. |
| 0.3 | Chưa tìm thấy bản hoặc SHA có thể xác minh. Không suy diễn nội dung. |
| 0.4 | Bản nháp quan sát trong working tree. Bổ sung wire mục tiêu, revision, lifecycle và yêu cầu pin Croc. Chưa xác nhận commit riêng. |
| 0.5 | Yêu cầu và RTM tách tiêu chí. ICD thêm đề nghị nhận, trạng thái hủy và mã lỗi, bỏ trường tiến độ trùng. AD chốt an toàn staging, so sánh engine và cổng spike. Thêm TM. Cập nhật V&V, CMP và thủ tục phát hành để tách các mức nghiệm thu. |
| 0.6 | Đồng bộ hồ sơ với tích hợp Windows (8 Wails binding theo ICD, wire byte dạng chuỗi thập phân không gửi `progress`/`bytesPerSecond`, bỏ `Idle` khỏi Go wire, `SingleInstanceLock`, Job Object kill-on-close, `CopySendCode` dùng message-only HWND, cập nhật bằng chứng kiểm thử Windows Go 1.27.4 và Frontend). |

Phiên bản chi tiết tại đầu từng file. Dùng Git để xem lịch sử từng file và bảng này để xác định baseline. Chỉ chỉnh tài liệu sở hữu thông tin bị thay đổi, rồi rà liên kết truy vết liên quan.
