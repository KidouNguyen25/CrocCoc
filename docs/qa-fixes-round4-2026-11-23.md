# CrocCoc — Bàn giao sửa QA đợt 4 (2026-11-23)

**Nguồn:** `croccoc-qa-review-round4.md` (Flowborn/Yue). **Checkout:** `D:\01-projects\CrocCoc`, `main`, các thay đổi vẫn ở working tree. **Phân công:** đội triển khai sửa logic/docs/build; đội QA độc lập chạy kiểm thử, review và xác nhận. Đây không phải giấy nghiệm thu phát hành.

| ID | Trạng thái triển khai | Sửa đổi / chốt logic |
| --- | --- | --- |
| **P1-QA-01** | Đã sửa, chờ QA | `frontend/src/App.tsx`: tách lỗi tracking/cancel khỏi lỗi chung; đọc kết quả boolean của `cancelTransfer`; re-probe snapshot sau yêu cầu hủy; nếu yêu cầu hủy thất bại mà vẫn mất tracking thì hiển thị **Buộc đặt lại giao diện** cùng cảnh báo backend có thể vẫn `BUSY`. Snapshot/event hợp lệ dọn lỗi hủy và ẩn nút cưỡng bức. Bình thường không reset phiên đang chạy. |
| **P1-QA-02** | Đã sửa, chờ QA | `docs/e2e-runbook.md`: cấu hình `CROCCOC_RELAY_ADDRESS` và `CROCCOC_RELAY_PASSWORD` trên cả hai máy **trước khi mở EXE**; ví dụ PowerShell nhập mật khẩu ẩn, không đưa bí mật vào command line, log hoặc bằng chứng; cập nhật CC-T-11 cho tình huống thiếu relay và worker. |
| **P2-QA-03** | Đã sửa, chờ QA | `.gitignore`: thêm `/release/` để đầu ra đóng gói không khiến lần build tiếp theo bị chặn bởi kiểm tra Git dirty. |
| **P2-QA-04** | Đã cải thiện, một số cổng vẫn thuộc QA/release | `scripts/build-release.ps1`: có `go vet`, `govulncheck`, `npm audit --audit-level=high` (fail-closed nếu không đạt); tạo CycloneDX SBOM Go từ **hai EXE thực tế** và frontend từ `npm sbom`; đưa SBOM vào SHA256SUMS; đọc Wails version từ `go.mod`, Croc version và upstream commit từ pin đang theo dõi; `qualityGatesSkipped` trong manifest. Worker nội bộ vẫn tên `croccoc-engine-spike.exe`, **gói phân phối** có `bin/croccoc-engine.exe` và kiểm hash pin. Chưa ký, chưa kiểm license/Authenticode/E2E. |
| **P2-QA-05** | Bàn giao QA | Cần test DOM/component cho lost-tracking, cancellation false/error, reset cưỡng bức và recovery; đội QA phụ trách theo phân công, đội triển khai không tự ghi nhận PASS. |
| **P2-QA-06** | Bàn giao QA | Cần smoke binary Croc **thật** trên Windows có bộ env allowlist tối giản; worker không nhận `USERPROFILE`/`APPDATA`/`LOCALAPPDATA` trừ khi QA chứng minh bắt buộc và mô hình đe dọa đã được xem xét. |
| **P2-QA-07** | Đã sửa, chờ QA | Backend `GetTransferUpdate`/event mang bytes trong `FINALIZING` và `COMPLETED`, nhớ metric của terminal gần nhất sau `releaseSession`; `SendPanel` lọc trùng đường dẫn hoặc tên root ở mọi cách thêm; `ReceivePanel` đặt lại `Downloads` khi chuyển sang mock; `ReceiveOfferDialog` bỏ `disabled` khỏi effect quản lý focus, tránh restore/refocus giữa phiên. |

## Các tình huống QA cần xác nhận

- Tracking mất, `cancelTransfer` trả `false` và snapshot `null`: hiển thị nút buộc reset; chỉ xóa UI, không tuyên bố backend đã hủy. Tracking trở lại trước/sau cancel lỗi: lỗi tạm biến mất, nút buộc reset không còn.
- Hủy trả `true`: vẫn chờ event terminal hoặc snapshot; không tự reset. Đang `FINALIZING` trả `false` và snapshot khỏe: không mở buộc reset.
- Đến `COMPLETED` với số byte đã có: `transfer:update` lẫn `GetTransferSnapshot` sau release giữ metrics; phiên mới không kế thừa dữ liệu của phiên cũ.
- Trùng tên gốc khác đường dẫn ở manual, drop, input file và native picker; chuyển engine Desktop → Mock; cập nhật `disabled` của offer dialog không đánh cắp focus.
- Build mới theo `scripts/build-release.ps1` trên máy Windows đủ Go/Node/Wails/govulncheck: output có `sbom-go.cdx.json` và `sbom-frontend.cdx.json`, worker đúng pin, manifest đánh dấu đúng `-SkipTests`, `release/` ignored. Phát hành vẫn cần kiểm độc lập audit/license/SBOM, chữ ký và E2E.

**Trạng thái xác minh:** QA vòng 4 đã báo Go/frontend pass **trước các sửa đổi mới này**. Theo yêu cầu phân công, lần triển khai này không chạy lại các bộ test; mọi mục sửa vẫn chờ QA xác nhận. **Chưa commit/push.**
