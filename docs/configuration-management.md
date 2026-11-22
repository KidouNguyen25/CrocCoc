# CMP — Quản lý cấu hình và thay đổi

**CC-CM-001 · 0.5 · 2026-11-19 · Dự thảo.** Tham chiếu [ISO/IEC/IEEE 15289:2019](https://www.iso.org/standard/74909.html).

| Hạng mục | Định danh |
| --- | --- |
| Nguồn và công cụ | Git SHA, Go module/sum, npm lockfile, phiên bản Go/Node/npm/Wails thực tế. |
| Hồ sơ kỹ thuật | Mã tài liệu, revision và lịch sử tập trung tại [danh mục](README.md). Git giữ diff theo từng tệp. |
| Croc và phụ thuộc | Pin spike `11.6.4` khớp init. Lưu tag, nguồn và checksum/chữ ký upstream nếu có. Nếu dùng helper, lưu ref patch, giấy phép MIT và dependency lock. Hash tự tính chỉ định danh bản đã tải, không tự chứng thực nguồn. Không dò binary từ PATH. |
| Phiên bản ứng dụng | SemVer. Release ứng dụng dùng Git tag `vX.Y.Z` cùng version trong `wails.json`, `frontend/package.json` và lockfile. Revision hồ sơ là dãy riêng. |
| Gói và chứng cứ | Hash gói cuối, binary đã chạy, SBOM, toolchain, Windows/WebView2, kết quả kiểm tra, nguồn và người chốt. Không đưa secret hoặc ZIP mockup vào baseline phát hành. |

Mã nguồn init là `610b622`. `e7e46ad` chỉnh hồ sơ 0.2 và không phải bản phát hành. 0.4 được quan sát trong working tree trước đợt chỉnh này. Không có SHA 0.3 được xác nhận trong lịch sử repo đã xem.

Khi sửa, ghi lý do và ID bị ảnh hưởng, cập nhật tài liệu sở hữu yêu cầu đó và các liên kết/ca thực sự đổi. Không đồng bộ nguyên văn qua mọi file. Tách namespace ticket `TASK-*`, spike `SPIKE-*`, yêu cầu `CC-FR-*`/`CC-NFR-*`, API `CC-API-*`, kiểm thử `CC-T-*`. CC-01–CC-08 chỉ là bí danh lịch sử, chưa xác minh với Notion. Commit tiếng Anh theo `type(scope): detail`.

## Nâng cấp Croc

Trước mỗi ứng viên và khi có cảnh báo mới, người phụ trách tích hợp rà advisory upstream, Go vulnerability database và phụ thuộc, ghi phiên bản bị ảnh hưởng, mức độ, khả năng tác động và quyết định. Không tự cập nhật từ engine lúc truyền. Nếu bản đang pin có lỗi chặn, giữ bản phân phối bị ảnh hưởng khỏi phát hành và đánh giá pin sửa lỗi. Bump có review, xác minh provenance/hash mới, chạy lại spike, ca native/relay và phát hành với SBOM mới. Nếu chưa có bản vá, ghi rủi ro và thay đổi phạm vi hoặc hoãn phát hành thay vì âm thầm bỏ qua.
