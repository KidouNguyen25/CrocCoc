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

2. Yêu cầu hệ thống & Khởi chạy nhanh
-------------------------------------
- Windows 10 hoặc Windows 11 (64-bit / x64) có Microsoft Edge WebView2 Evergreen Runtime.
- Mặc định, bạn chỉ cần giải nén gói và mở trực tiếp CrocCoc.exe để sử dụng ngay:
  ứng dụng được cấu hình sẵn tuyến relay công cộng chính thức của Croc v11.6.4
  ("croc.schollz.com:9009" với mật khẩu công khai mặc định của upstream Croc).
- Giao diện sẽ hiển thị rõ nhãn tuyến đang dùng:
  "Relay công cộng: croc.schollz.com:9009" (hoặc "Relay tùy chỉnh" nếu bạn đặt
  relay riêng theo mục 3 bên dưới).

3. Lưu ý quyền riêng tư, giới hạn Relay công cộng & Tự dựng Relay riêng
-----------------------------------------------------------------------
- Quyền riêng tư: Nội dung tệp được mã hóa đầu cuối theo giao thức Croc PAKE
  ngay từ máy gửi đến máy nhận. Máy chủ relay chỉ quan sát được metadata mạng
  (địa chỉ IP, thời điểm, dung lượng truyền) và KHÔNG đọc được nội dung tệp.
- Giới hạn của relay công cộng bên thứ ba ("croc.schollz.com:9009"):
  + Đây là hạ tầng công cộng của bên thứ ba nên tốc độ có thể chậm hoặc quá tải
    vào giờ cao điểm.
  + Croc sử dụng kết nối TCP trên cổng 9009 (và các cổng dữ liệu phụ 9010–9013),
    do đó tường lửa mạng công ty, trường học hoặc VPN hạn chế cổng TCP lạ có thể
    chặn kết nối tới relay. Khi không kết nối được, ứng dụng sẽ thông báo rõ lỗi
    không tới được máy chủ relay.
- Cách tự dựng máy chủ Relay riêng và ghi đè cấu hình (Fail-Closed):
  + Bạn có thể tự vận hành relay Croc trên máy chủ/VPS riêng có mở cổng TCP
    9009–9013 (ví dụ: croc relay --ports 9009,9010,9011,9012,9013 --pass "<mật-khẩu-riêng>").
  + Để dùng relay riêng, hãy đặt ĐỦ CẢ HAI biến môi trường sau trước khi mở
    CrocCoc.exe:
      * CROCCOC_RELAY_ADDRESS  : Địa chỉ máy chủ relay (ví dụ "relay.example.com:9009").
      * CROCCOC_RELAY_PASSWORD : Mật khẩu kết nối máy chủ relay của bạn.

    Ví dụ khởi chạy với relay riêng từ Windows PowerShell:

      $env:CROCCOC_RELAY_ADDRESS = "<địa-chỉ-relay:cổng>"
      $env:CROCCOC_RELAY_PASSWORD = "<mật-khẩu-relay-của-bạn>"
      .\CrocCoc.exe

  + Cơ chế bảo vệ Fail-Closed:
      * Không đặt cả hai biến -> Dùng relay công cộng mặc định ("croc.schollz.com:9009").
      * Đặt đủ cả hai biến    -> Dùng relay tùy chỉnh của bạn ("Relay tùy chỉnh").
      * Chỉ đặt 1 trong 2 biến (thiếu địa chỉ hoặc thiếu mật khẩu) -> Ứng dụng
        từ chối khởi tạo engine (ENGINE_UNAVAILABLE) kèm thông báo lỗi rõ ràng,
        tuyệt đối không lẳng lặng rơi về relay công cộng khi bạn cấu hình dở.
  + Khi khởi động, CrocCoc.exe đọc CROCCOC_RELAY_PASSWORD vào bộ nhớ rồi xóa ngay
    biến này khỏi môi trường tiến trình chính, và chỉ truyền cho tiến trình con
    bin\croccoc-engine.exe trong danh sách biến môi trường tối thiểu.

4. Kiểm tra toàn vẹn gói & Lưu ý bản Pre-release MVP
----------------------------------------------------
- Kiểm tra mã băm SHA-256 của các tệp bằng PowerShell và đối chiếu với SHA256SUMS.txt:

    Get-FileHash .\CrocCoc.exe, .\bin\croccoc-engine.exe -Algorithm SHA256

- Bản phát hành thử nghiệm (Pre-release MVP) chưa ký chữ ký số Authenticode nên
  Windows SmartScreen có thể hiển thị cảnh báo với tệp thực thi mới tải về. Hãy
  xác minh SHA256SUMS.txt khớp với bản công bố trên trang GitHub Release trước
  khi chạy.
