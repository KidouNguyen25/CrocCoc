# Runbook nghiệm thu E2E và demo CrocCoc (CC-08)

Tài liệu thao tác cho [V&V](verification.md) CC-T-11 đến CC-T-16. Ghi kết quả vào mục 4 của V&V theo mẫu hồ sơ chạy; không đánh dấu `PASS` trước khi chạy thật.

## 1. Chuẩn bị

1. Build gói: `scripts\build-release.ps1` (máy Windows x64). Ghi SHA-256 của `CrocCoc-<ver>-win64.zip` và `git SHA` trong `release-manifest.json`.
2. Hai máy Windows 10/11 x64 có WebView2. Giải nén gói vào thư mục mới, giữ `CrocCoc.exe` cạnh `bin\croccoc-engine.exe`. Không cần đặt `CROCCOC_WORKER_PATH`.
3. **Bắt buộc cấu hình relay trên cả hai máy trước khi mở EXE.** Host chỉ báo engine khả dụng nếu `CROCCOC_RELAY_ADDRESS` (dạng `host:port`) và `CROCCOC_RELAY_PASSWORD` đều khác rỗng. Dùng cùng thông tin relay đã cấp phép trên hai máy; mật khẩu relay khác mã kết nối truyền. Mở PowerShell **tại thư mục vừa giải nén**, chạy các lệnh sau và luôn khởi chạy ứng dụng từ phiên PowerShell này:

```powershell
$env:CROCCOC_RELAY_ADDRESS = Read-Host 'Địa chỉ relay (host:port)'
$secure = Read-Host 'Mật khẩu relay' -AsSecureString
$ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
try { $env:CROCCOC_RELAY_PASSWORD = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr) }
finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr); Remove-Variable secure -ErrorAction SilentlyContinue }
try { & .\CrocCoc.exe }
finally {
    Remove-Item Env:CROCCOC_RELAY_ADDRESS -ErrorAction SilentlyContinue
    Remove-Item Env:CROCCOC_RELAY_PASSWORD -ErrorAction SilentlyContinue
}
```

Không ghi mật khẩu thật trong command line, file `.env`, transcript, log, ảnh hay hồ sơ QA. Tránh bật PowerShell transcription khi nhập credential; không chụp cửa sổ có thông tin nhạy cảm. Nếu thiếu relay/password, `ENGINE_UNAVAILABLE` là hành vi **đúng** (fail-closed), không phải lỗi pin worker.
4. Trên máy gửi tạo fixture (Node 24+ ở máy phát triển, không cần cài trên máy demo nếu copy sẵn fixture):

```powershell
node scripts\qa\fixtures.mjs make C:\qa\src --large-mb 512
```

Fixture gồm tệp nhỏ, cây thư mục tên Unicode có khoảng trắng và thư mục rỗng, và `large.bin`. Manifest sinh ra tại `C:\qa\src.manifest.json`. Ghi số byte từng tệp vào hồ sơ trước khi chạy.

## 2. Đối chiếu sau mỗi lần nhận

```powershell
node scripts\qa\fixtures.mjs compare C:\qa\src\small.txt D:\qa\received-small\small.txt
node scripts\qa\fixtures.mjs compare "C:\qa\src\Thư mục thử nghiệm" "D:\qa\received-folder\Thư mục thử nghiệm"
node scripts\qa\fixtures.mjs compare C:\qa\src\large.bin D:\qa\received-large\large.bin
```

Kết luận `PASS` khi mọi tệp nguồn có mặt với cùng kích thước và SHA-256, mọi thư mục rỗng được giữ. Lệnh thoát mã 1 khi lệch. Chạy với đích sạch cho ca mặc định. Không đưa mã kết nối vào ảnh hay log lưu làm bằng chứng.

## 3. Thao tác theo ca

| Ca | Thao tác | Ghi lại |
| --- | --- | --- |
| CC-T-11 | Cấu hình hai biến relay theo bước 3 trên cả hai máy rồi mới mở `CrocCoc.exe` từ PowerShell. Đọc trạng thái Croc. Thử mở thiếu relay/password (phải fail-closed), đổi tên `bin\croccoc-engine.exe`, rồi thay bằng bản bị sửa một byte; khôi phục bản gốc và cấu hình relay trước khi thử lại. | Có relay hợp lệ + đúng worker pin thì khả dụng; thiếu relay, thiếu worker hoặc sai hash thì không khả dụng. Không lưu relay password hay mã kết nối trong bằng chứng. |
| CC-T-12 | Gửi `small.txt`, rồi gửi cả `Thư mục thử nghiệm` kèm `small.txt`, bỏ `small.txt` trước khi gửi, nhận bằng mã. Kiểm thư mục kết quả bằng Explorer (nút mở thư mục từ app chưa có trong MVP). | Ảnh đã che mã và kết quả `compare` cho từng lượt, dùng đích sạch riêng theo ca và đường dẫn nguồn/đích tương ứng. |
| CC-T-13 | Hủy khi chờ người nhận và khi đang truyền ở cả hai vai. Mở phiên thứ hai khi phiên đầu còn chạy. Kiểm tra `Get-Process` không còn tiến trình con. | Dòng thời gian trạng thái, kết quả `Get-Process`. |
| CC-T-14 | Nhập mã sai, chọn đích không ghi được, xóa nguồn giữa chừng, rút mạng giữa lúc truyền. Rà `%LOCALAPPDATA%`, thông báo UI và log để tìm mã kết nối. | Bước tái hiện, kết luận rà bí mật. |
| CC-T-15 | Truyền `large.bin` giữa hai mạng khác nhau qua relay đã xác nhận (hai máy không cùng LAN). | `compare` PASS, thông tin xác nhận đường relay không chứa mã. |
| CC-T-16 | Chạy đúng gói đã tính SHA-256 trên Windows 10 và Windows 11. | SHA-256 artifact, bản Windows, bản WebView2, kết quả. |

### Kiểm chứng an toàn Windows bắt buộc trước sign-off

- **CC-T-14B / Clipboard:** trên cả hai máy bật Win+V và Cloud Clipboard, thử ghi marker thông thường để đối chứng history và đồng bộ đang hoạt động. Sau đó bấm **Sao chép mã** trong CrocCoc và xác minh dán được mã hiện tại, nhưng mã không xuất hiện trong Win+V và không đến máy thứ hai. Không chụp hoặc lưu mã thật trong bằng chứng.
- **CC-T-12A/B, CC-T-13B / NTFS:** chạy trên cả C: và D: (NTFS). Thử tên thiết bị, ADS, alias 8.3, symlink/reparse point và thay ancestor giữa lúc đồng ý và publish. Ghi nhận từ chối đúng lỗi, không ghi đè, không mở payload trước consent; kiểm hủy giữa chừng, đóng app khi `TRANSFERRING` hoặc `FINALIZING`, restart và kiểm tài nguyên/staging.
- **CC-T-15B / Relay:** đặt 2 máy ở 2 mạng khác nhau; ghi lại endpoint thực của kết nối/relay bằng công cụ hệ thống, che mã và credential. Không chấp nhận bằng chứng chạy mock hay LAN.
- **CC-T-16A/B / Máy sạch:** thử Windows 10 và 11 x64 bằng cùng SHA-256 gói ứng viên; thử thiếu WebView2, cài Evergreen Bootstrapper hoặc Standalone offline, và kiểm SmartScreen/Defender/Firewall ở cấu hình mặc định. Không yêu cầu người dùng tắt bảo vệ.
- **Phát hành:** kiểm SBOM, license, nguồn/hash worker và WebView2; ký Authenticode có timestamp trước khi công bố công khai. Script hiện chỉ tạo **ứng viên chưa ký**, không phải bản phát hành đã nghiệm thu.

Ca còn nợ theo README của worker: khôi phục sau khi khởi động lại Windows giữa lúc nhận (journal staging), và xác nhận gói cuối dùng đúng worker đã ghim. Hai mục này cần ca riêng trong hồ sơ chạy.

## 4. Kịch bản demo (khoảng 5 phút)

1. Máy A: mở CrocCoc, chọn thư mục `Thư mục thử nghiệm` và `small.txt`, bấm Gửi, sao chép mã.
2. Máy B: mở CrocCoc, nhập mã, chọn thư mục lưu, xem đề nghị nhận (danh sách tệp), chấp nhận.
3. Chạy `compare` trên máy nhận để chứng minh toàn vẹn.
4. Lượt hai: bắt đầu gửi `large.bin` rồi hủy giữa chừng, chỉ ra không còn tiến trình con.
5. Nhập mã sai để cho thấy thông báo lỗi dễ hiểu.

Giữ sẵn ảnh chụp và `release-manifest.json` của gói đang demo để đối chiếu nếu bị hỏi về phiên bản.
