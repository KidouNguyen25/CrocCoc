# Backend engine Croc

Backend ghim Croc v11.6.4 tại commit `f4cfdc17e06ac8a5834e146666798170138c3391`. Patch mở callback manifest trước khi Croc yêu cầu payload. Patch không đổi wire protocol. Không áp dụng patch vào checkout CrocCoc.

## Build và xác minh worker

Chạy PowerShell trên Windows:

```powershell
.\backend\engine-spike\worker\build.ps1
```

Script clone đúng commit Croc, kiểm rồi áp patch, khóa Go 1.27.4 và build Windows amd64 không Tailcat. Script chạy loopback test gồm nhận tệp và thư mục rỗng, rồi so SHA-256 với `backend/transfer/worker_hash_windows.go`. Build dừng nếu hash không khớp. Worker xuất hiện tại `backend/engine-spike/worker/croccoc-engine-spike.exe`; file build không thuộc Git.

Host nhận đường dẫn tuyệt đối qua `CROCCOC_WORKER_PATH`. Backend chỉ báo Croc khả dụng khi worker khớp SHA-256 đã ghim. Backend giữ khóa file và tổ tiên để chặn sửa hoặc thay worker trong lúc app chạy. Relay dùng `CROCCOC_RELAY_ADDRESS` và `CROCCOC_RELAY_PASSWORD`.

## Giao thức và luồng nhận

Worker dùng pipe riêng v2: mỗi frame là một JSON trên một dòng, tối đa 8 KiB. Mã kết nối, mật khẩu relay và đường dẫn nguồn đi qua stdin; không đưa vào argv, env của worker hoặc stdout. Stdout chỉ chứa frame có cấu trúc. Worker tắt local discovery và clipboard.

Worker gửi manifest đã chuẩn hóa cho host. Host kiểm tên, quota, đích và hiện đề nghị nhận. Chỉ quyết định đúng `offerID` mới kích hoạt staging. Worker tiếp tục Croc vào staging riêng; backend kiểm cây payload và publish từng mục gốc bằng rename no-replace. Worker từ chối, timeout hoặc lỗi trước khi staging được cấp quyền không ghi payload.

`ReceiveManifest` chỉ thu metadata. `ReceiveAndContinue` tiếp tục truyền sau khi callback của host cho phép. Ca loopback trên Windows kiểm cả hai nhánh. Nhánh nhận thật xác minh nội dung tệp và thư mục rỗng; các nhánh từ chối, sai `offerID` và đóng pipe xác minh không ghi payload.

## Kiểm thử Windows

Chạy backend tests tại repo root:

```powershell
go test -count=1 ./...
```

Build script cũng chạy worker tests từ checkout Croc sạch trước khi tạo EXE. Hai máy Windows qua relay công cộng và gói Wails cài sạch vẫn cần nghiệm thu riêng.

## Backend đã hoàn thiện

- Backend lưu đích nhận trong journal cục bộ có DACL riêng. Recovery kiểm marker và identity trước khi xóa stage còn lại. Test NTFS dừng tiến trình con đột ngột. Mục đã publish không bị rollback; journal sai hoặc đích offline chặn engine.
- Backend áp dụng 2 phút chuẩn bị, 5 phút chờ peer, 2 phút duyệt và 2 phút không có byte payload mới. Worker báo lúc peer chấp nhận và gửi byte delta qua pipe riêng. Backend chuyển `TRANSFERRING` đúng lúc gửi bắt đầu; UI chưa nhận tiến độ byte. Backend đợi xác nhận worker dừng tối đa 10 giây; nếu chưa xác nhận, backend giữ tài nguyên và chặn phiên mới.

## Cần nghiệm thu

- Xác minh recovery khi mở app sau reboot Windows.
- Xác minh tuyến relay công cộng trên hai máy và xác minh gói Wails cuối dùng đúng worker đã ghim.

Vì vậy, phần backend đã hoàn thiện và qua loopback Windows; các mục nghiệm thu trên vẫn còn.
