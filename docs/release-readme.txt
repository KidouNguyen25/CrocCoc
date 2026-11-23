CrocCoc cho Windows x64 — Hướng dẫn sử dụng & Cấu hình Relay
============================================================

1. Thành phần trong gói
-----------------------
- CrocCoc.exe               : Ứng dụng desktop CrocCoc (Wails / WebView2).
- bin\croccoc-engine.exe    : Tiến trình worker Croc v11.6.4 đã ghim mã băm SHA-256.
- LICENSE                   : Giấy phép Apache-2.0 của CrocCoc.
- THIRD_PARTY_LICENSES.txt  : Giấy phép phần mềm bên thứ ba (gồm MIT License của Croc).
- sbom-go.cdx.json          : Danh mục thành phần CycloneDX của các tệp thực thi Go.
- sbom-frontend.cdx.json    : Danh mục thành phần CycloneDX của giao diện frontend.
- SHA256SUMS.txt            : Bảng mã băm SHA-256 của toàn bộ tệp trong gói.
- release-manifest.json     : Thông tin bản dựng, phiên bản công cụ và mã băm đã ghim.

Lưu ý: Để chống tấn công nạp nhầm tệp thực thi (binary planting), CrocCoc.exe
chỉ nạp worker tại đúng đường dẫn tương đối "bin\croccoc-engine.exe" nằm cạnh
CrocCoc.exe (hoặc đường dẫn tuyệt đối do bạn chỉ định qua CROCCOC_WORKER_PATH)
và luôn xác minh mã băm SHA-256 trước khi chạy.

2. Yêu cầu hệ thống
-------------------
- Windows 10 hoặc Windows 11 (64-bit / x64).
- Microsoft Edge WebView2 Evergreen Runtime (thường có sẵn trên Windows 10/11).

3. Cấu hình máy chủ Relay trước khi chạy (BẮT BUỘC)
---------------------------------------------------
Vì lý do bảo mật, CrocCoc không nhúng sẵn địa chỉ hay mật khẩu máy chủ relay vào
mã nguồn hoặc gói phát hành. Bạn cần đặt hai biến môi trường sau trước khi mở
CrocCoc.exe; nếu thiếu một trong hai biến, ứng dụng sẽ báo Engine chưa sẵn sàng:

- CROCCOC_RELAY_ADDRESS  : Địa chỉ máy chủ relay Croc (ví dụ "relay.example.com:9009" hoặc "127.0.0.1:9009").
- CROCCOC_RELAY_PASSWORD : Mật khẩu kết nối máy chủ relay của bạn.

Cách chạy từ Windows PowerShell:

    $env:CROCCOC_RELAY_ADDRESS = "<địa-chỉ-relay:cổng>"
    $env:CROCCOC_RELAY_PASSWORD = "<mật-khẩu-relay-của-bạn>"
    .\CrocCoc.exe

(Nếu muốn đặt cố định cho tài khoản người dùng Windows hiện tại, bạn có thể mở
System Properties -> Environment Variables -> User variables để thêm hai biến
CROCCOC_RELAY_ADDRESS và CROCCOC_RELAY_PASSWORD, sau đó mở lại CrocCoc.exe).

Khi khởi động, CrocCoc.exe đọc CROCCOC_RELAY_PASSWORD vào bộ nhớ rồi xóa ngay
biến này khỏi môi trường tiến trình chính, và chỉ truyền cho tiến trình con
bin\croccoc-engine.exe trong phạm vi tối thiểu khi thực hiện phiên gửi/nhận.

4. Kiểm tra toàn vẹn gói & Lưu ý bản Pre-release MVP
----------------------------------------------------
- Bạn có thể kiểm tra mã băm SHA-256 của các tệp bằng PowerShell và đối chiếu
  với SHA256SUMS.txt:

    Get-FileHash .\CrocCoc.exe, .\bin\croccoc-engine.exe -Algorithm SHA256

- Bản phát hành thử nghiệm (Pre-release MVP) chưa ký chữ ký số Authenticode nên
  Windows SmartScreen có thể hiển thị cảnh báo với tệp thực thi mới tải về. Hãy
  xác minh SHA256SUMS.txt khớp với bản công bố trên trang GitHub Release trước
  khi chạy.
