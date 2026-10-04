# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: attribution.spec.ts >> AT7/AT9 actual report clicks zh-TW 390
- Location: tests/admin/attribution.spec.ts:646:5

# Error details

```
Error: expect(locator).toHaveText(expected) failed

Locator: getByTestId('attribution-linked-drafts').locator('li')
Timeout: 10000ms
- Expected  - 2
+ Received  + 2

@@ -1,9 +1,9 @@
  Array [
-   "3c2ca8fd-8656-420d-abaf-9b071b716294",
-   "90372a4e-b32d-46a0-a044-1613d9464349",
    "5f6c84ee-8407-421f-8e16-05f5482bc9e2",
+   "90372a4e-b32d-46a0-a044-1613d9464349",
+   "3c2ca8fd-8656-420d-abaf-9b071b716294",
    "92232c3a-e9d8-41a9-9e7a-59ea75ff61ed",
    "34ecc085-6e32-4ea6-a809-12fd513cd28c",
    "9350b730-fff0-4af2-9a82-30ad7273fc24",
    "4532c929-19e7-46bc-aeba-300d40c7233e",
    "4053644e-d63e-49c2-8c1b-7c20b8ed4544",

Call log:
  - Expect "toHaveText" getByTestId('attribution-linked-drafts').locator('li') with timeout 10000ms
  - waiting for getByTestId('attribution-linked-drafts').locator('li')
    24 × locator resolved to 103 elements

```

# Page snapshot

```yaml
- generic [ref=f3e1]:
  - generic [ref=f3e2]:
    - link "跳至內容" [ref=f3e3] [cursor=pointer]:
      - /url: "#main"
    - generic [ref=f3e4]:
      - banner [ref=f3e5]:
        - button "開啟導覽" [ref=f3e6] [cursor=pointer]
        - generic [ref=f3e9]:
          - generic [ref=f3e10]: 切換商店
          - combobox "切換商店" [ref=f3e11] [cursor=pointer]:
            - option "payment mock store" [selected]
        - generic [ref=f3e12]:
          - generic [ref=f3e13]: 語言
          - combobox "語言" [ref=f3e14] [cursor=pointer]:
            - option "简体中文"
            - option "繁體中文" [selected]
            - option "English"
        - group [ref=f3e15]:
          - generic "說明" [ref=f3e16] [cursor=pointer]
        - group [ref=f3e17]:
          - generic "帳號" [ref=f3e18] [cursor=pointer]
      - main [ref=f3e19]:
        - navigation "工作區導覽" [ref=f3e20]:
          - link "總覽" [ref=f3e21] [cursor=pointer]:
            - /url: /zh-TW/
          - generic [aria-hidden] [ref=f3e22]: /
          - generic [ref=f3e23]: 行銷優惠
          - generic [aria-hidden] [ref=f3e24]: /
          - generic [ref=f3e25]: 廣告歸因
        - generic [ref=f3e26]:
          - generic [ref=f3e27]:
            - link "返回廣告" [ref=f3e28] [cursor=pointer]:
              - /url: /zh-TW/ads?store=648fbf9b-309e-4b61-a778-456a89ef3ebc
            - heading "廣告歸因與直播復盤" [level=1] [ref=f3e29]
            - paragraph [ref=f3e30]: 訂單實績與 Meta 歸因採不同口徑，數字可能不同，兩者不相加。
          - generic [ref=f3e31]:
            - generic [ref=f3e32]:
              - text: 開始日期
              - textbox "開始日期" [ref=f3e33]: 2026-10-04
            - generic [ref=f3e34]:
              - text: 結束日期
              - textbox "結束日期" [ref=f3e35]: 2026-10-04
            - button "讀取報表" [ref=f3e36] [cursor=pointer]
          - paragraph [ref=f3e37]: 2026-10-04 – 2026-10-04 · 訂單日期：Asia/Taipei
          - status [ref=f3e38]: 僅顯示前 100 筆
          - generic [ref=f3e39]:
            - generic [ref=f3e40]:
              - text: 廣告草稿
              - combobox "廣告草稿" [ref=f3e41]:
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149" [selected]
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
                - option "1254781134323423807_8128278149"
            - generic [ref=f3e42]:
              - text: 直播場次
              - combobox "直播場次" [ref=f3e43]:
                - option "claims gate c863d7c210e5"
                - option "claims gate ff94dd1cb999"
                - option "claims gate 729bab1f797b"
                - option "claims gate 516d5635e035"
                - option "claims gate 9227228e11ed"
                - option "claims gate d9a61da1f836"
                - option "claims gate ae04261ebc82"
                - option "claims gate a3239b1cc2ea"
                - option "claims gate 56f845953af8"
                - option "claims gate 4e01c18a8532"
                - option "claims gate 20fcf4ee446c" [selected]
          - region [ref=f3e44]:
            - heading "草稿報表" [level=2] [ref=f3e45]
            - paragraph [ref=f3e46]: "來源參照: 1254781134323423807_8128278149"
            - paragraph [ref=f3e47]: "Meta 帳戶時區: America/Los_Angeles"
            - status [ref=f3e48]: Meta 數據可能仍會更新
            - generic [ref=f3e49]:
              - generic [ref=f3e50]:
                - term [ref=f3e51]: 廣告花費
                - definition [ref=f3e52]: NT$12.30
              - generic [ref=f3e53]:
                - term [ref=f3e54]: 訂單 ROAS
                - definition [ref=f3e55]: 10.08×
            - generic [ref=f3e56]:
              - generic [ref=f3e57]:
                - heading "訂單實績" [level=3] [ref=f3e58]
                - paragraph [ref=f3e59]: 受推廣貼文的訂單可能來自付費或自然留言，無法確認廣告曝光。
                - region "來源路徑 · 已收款訂單 · 扣除退款後淨營收 · 待收款訂單 · 待收款金額" [ref=f3e60]:
                  - table [ref=f3e61]:
                    - rowgroup [ref=f3e62]:
                      - row [ref=f3e63]:
                        - columnheader "來源路徑" [ref=f3e64]
                        - columnheader "已收款訂單" [ref=f3e65]
                        - columnheader "扣除退款後淨營收" [ref=f3e66]
                        - columnheader "待收款訂單" [ref=f3e67]
                        - columnheader "待收款金額" [ref=f3e68]
                    - rowgroup [ref=f3e69]:
                      - row [ref=f3e70]:
                        - rowheader "廣告點擊" [ref=f3e71]
                        - cell "2" [ref=f3e72]
                        - cell "NT$49" [ref=f3e73]
                        - cell "0" [ref=f3e74]
                        - cell "NT$0" [ref=f3e75]
                      - row [ref=f3e76]:
                        - rowheader "受推廣貼文帶來" [ref=f3e77]
                        - cell "1" [ref=f3e78]
                        - cell "NT$75" [ref=f3e79]
                        - cell "0" [ref=f3e80]
                        - cell "NT$0" [ref=f3e81]
              - generic [ref=f3e82]:
                - heading "Meta 回報" [level=3] [ref=f3e83]
                - generic [ref=f3e84]:
                  - generic [ref=f3e85]:
                    - term [ref=f3e86]: 購買次數
                    - definition [ref=f3e87]: "9"
                  - generic [ref=f3e88]:
                    - term [ref=f3e89]: 購買金額
                    - definition [ref=f3e90]: NT$900
            - heading "廣告受眾（Meta 統計）" [level=3] [ref=f3e91]
            - paragraph [ref=f3e92]: "2026-10-04 · 無法取得分類資料: 年齡與性別"
            - region "Meta 帳戶日期 · Meta 帳戶時區 · 分類 · 群組 · 時間（Asia/Taipei） · 廣告花費 · 觸及 · 曝光 · 點擊 · 互動 · 留言 · 購買次數 · 購買金額" [ref=f3e93]:
              - table [ref=f3e94]:
                - rowgroup [ref=f3e95]:
                  - row [ref=f3e96]:
                    - columnheader "Meta 帳戶日期" [ref=f3e97]
                    - columnheader "Meta 帳戶時區" [ref=f3e98]
                    - columnheader "分類" [ref=f3e99]
                    - columnheader "群組" [ref=f3e100]
                    - columnheader "時間（Asia/Taipei）" [ref=f3e101]
                    - columnheader "廣告花費" [ref=f3e102]
                    - columnheader "觸及" [ref=f3e103]
                    - columnheader "曝光" [ref=f3e104]
                    - columnheader "點擊" [ref=f3e105]
                    - columnheader "互動" [ref=f3e106]
                    - columnheader "留言" [ref=f3e107]
                    - columnheader "購買次數" [ref=f3e108]
                    - columnheader "購買金額" [ref=f3e109]
                - rowgroup [ref=f3e110]:
                  - row [ref=f3e111]:
                    - cell "2026-10-04" [ref=f3e112]
                    - cell "America/Los_Angeles" [ref=f3e113]
                    - cell "每小時" [ref=f3e114]
                    - rowheader "13:00:00 - 13:59:59" [ref=f3e115]
                    - cell "2026/10/05 04:00" [ref=f3e116]
                    - cell "NT$12.30" [ref=f3e117]
                    - cell "未提供" [ref=f3e118]
                    - cell "未提供" [ref=f3e119]
                    - cell "未提供" [ref=f3e120]
                    - cell "未提供" [ref=f3e121]
                    - cell "未提供" [ref=f3e122]
                    - cell "未提供" [ref=f3e123]
                    - cell "未提供" [ref=f3e124]
            - generic [ref=f3e125]:
              - heading "買家分佈（訂單實績）" [level=3] [ref=f3e126]
              - generic [ref=f3e127]:
                - generic [ref=f3e128]:
                  - term [ref=f3e129]: 新買家
                  - definition [ref=f3e130]: "3"
                - generic [ref=f3e131]:
                  - term [ref=f3e132]: 回購買家
                  - definition [ref=f3e133]: "0"
                - generic [ref=f3e134]:
                  - term [ref=f3e135]: 平均訂單金額
                  - definition [ref=f3e136]: NT$41.33
              - region "收件縣市 · 已收款訂單 · 扣除退款後淨營收" [ref=f3e137]:
                - table [ref=f3e138]:
                  - rowgroup [ref=f3e139]:
                    - row [ref=f3e140]:
                      - columnheader "收件縣市" [ref=f3e141]
                      - columnheader "已收款訂單" [ref=f3e142]
                      - columnheader "扣除退款後淨營收" [ref=f3e143]
                  - rowgroup [ref=f3e144]:
                    - row [ref=f3e145]:
                      - rowheader "臺北市" [ref=f3e146]
                      - cell "3" [ref=f3e147]
                      - cell "NT$124" [ref=f3e148]
              - heading "熱銷商品" [level=4] [ref=f3e149]
              - region "商品 · 數量" [ref=f3e150]:
                - table [ref=f3e151]:
                  - rowgroup [ref=f3e152]:
                    - row [ref=f3e153]:
                      - columnheader "商品" [ref=f3e154]
                      - columnheader "數量" [ref=f3e155]
                  - rowgroup [ref=f3e156]:
                    - row [ref=f3e157]:
                      - rowheader "t04-ce8a746d435d" [ref=f3e158]
                      - cell "6" [ref=f3e159]
              - heading "每分鐘訂單" [level=4] [ref=f3e160]
              - region "時間（Asia/Taipei） · 已收款訂單 · 扣除退款後淨營收" [ref=f3e161]:
                - table [ref=f3e162]:
                  - rowgroup [ref=f3e163]:
                    - row [ref=f3e164]:
                      - columnheader "時間（Asia/Taipei）" [ref=f3e165]
                      - columnheader "已收款訂單" [ref=f3e166]
                      - columnheader "扣除退款後淨營收" [ref=f3e167]
                  - rowgroup [ref=f3e168]:
                    - row [ref=f3e169]:
                      - rowheader "2026/10/04 14:57" [ref=f3e170]
                      - cell "3" [ref=f3e171]
                      - cell "NT$124" [ref=f3e172]
          - region [ref=f3e173]:
            - 'heading "直播復盤: claims gate 20fcf4ee446c" [level=2] [ref=f3e174]'
            - paragraph [ref=f3e175]: "開始: 2026/10/04 14:57 · 結束: 未提供"
            - paragraph [ref=f3e176]: "綁定貼文: 1254781134323423807_8128278149"
            - group [ref=f3e177]:
              - generic "關聯草稿 (103)" [active] [ref=f3e178] [cursor=pointer]
              - list [ref=f3e179]:
                - listitem [ref=f3e180]: 5f6c84ee-8407-421f-8e16-05f5482bc9e2
                - listitem [ref=f3e181]: 90372a4e-b32d-46a0-a044-1613d9464349
                - listitem [ref=f3e182]: 3c2ca8fd-8656-420d-abaf-9b071b716294
                - listitem [ref=f3e183]: 92232c3a-e9d8-41a9-9e7a-59ea75ff61ed
                - listitem [ref=f3e184]: 34ecc085-6e32-4ea6-a809-12fd513cd28c
                - listitem [ref=f3e185]: 9350b730-fff0-4af2-9a82-30ad7273fc24
                - listitem [ref=f3e186]: 4532c929-19e7-46bc-aeba-300d40c7233e
                - listitem [ref=f3e187]: 4053644e-d63e-49c2-8c1b-7c20b8ed4544
                - listitem [ref=f3e188]: 94047c19-bce0-4ce2-a275-690ce623c102
                - listitem [ref=f3e189]: ebaf2f47-bb16-4b76-a9bc-0d5fb79692ed
                - listitem [ref=f3e190]: a2a9848f-ca74-4a21-88c3-265109958c43
                - listitem [ref=f3e191]: 2360488e-bfc5-43ab-b086-221b8ba245fc
                - listitem [ref=f3e192]: 650a4218-73e4-4a46-9fcc-bc011cf45099
                - listitem [ref=f3e193]: d3cea11e-1a19-4932-a490-1ac585515cb8
                - listitem [ref=f3e194]: 563e24c6-4946-492b-b4d7-2dd4b40b6d70
                - listitem [ref=f3e195]: 13c51fd7-0b7a-4d5d-a861-764fcacdf602
                - listitem [ref=f3e196]: 29df493e-7aa2-4397-8933-2f655362d3be
                - listitem [ref=f3e197]: cb71605a-fb85-42d3-9699-fb1d3c89efcf
                - listitem [ref=f3e198]: 08923d6b-5d91-4b7c-8278-aefdef3c5850
                - listitem [ref=f3e199]: 8ab58155-066a-430e-a980-4c4c318f9654
                - listitem [ref=f3e200]: d734a9a4-9073-4ae3-aa55-bd3fc29d4b9c
                - listitem [ref=f3e201]: eaf6bb90-72bf-4d16-82b7-c5777197cb2d
                - listitem [ref=f3e202]: 9c14fb5c-4e1c-40d6-9ee0-131343d252d0
                - listitem [ref=f3e203]: de789e60-b1d0-4919-a406-cb2401e91146
                - listitem [ref=f3e204]: 94d37df3-5b51-4824-b5cc-4c06df50178c
                - listitem [ref=f3e205]: acd307e7-387b-4ddd-b453-592e987497cf
                - listitem [ref=f3e206]: de14f1cd-bf19-4cf6-9c46-df13c5fc8bde
                - listitem [ref=f3e207]: dc061060-f937-4f2f-bc2b-4c26fb647556
                - listitem [ref=f3e208]: 0f4c2b0f-2985-4287-9cef-845253e1f0f9
                - listitem [ref=f3e209]: 106994b2-ff8a-4357-aa7f-96cf79d70bc0
                - listitem [ref=f3e210]: 94f3c801-838e-47a6-a35c-3e4162b27b1f
                - listitem [ref=f3e211]: 4b25eec7-86cf-419d-9ba2-eff74e1cf0b7
                - listitem [ref=f3e212]: 1c0523f7-81df-47da-9469-147e6e5ab57d
                - listitem [ref=f3e213]: 18ec82f2-5e10-473d-aab0-0f24dd0bf13e
                - listitem [ref=f3e214]: 26547c84-e406-4027-9029-9db365410f62
                - listitem [ref=f3e215]: d073d009-c443-4902-a0ee-5e1842a11bd6
                - listitem [ref=f3e216]: 62ff1a90-f265-490f-bd82-af5904419454
                - listitem [ref=f3e217]: 3144ea62-58ab-4a61-9a90-390ff466f3b6
                - listitem [ref=f3e218]: fcbd0e62-6adb-4e17-8e24-eb21fe9968b7
                - listitem [ref=f3e219]: d72bb9c5-6872-471d-b22d-216f92684d9f
                - listitem [ref=f3e220]: 495a91a8-6954-4755-8056-d413651a0d06
                - listitem [ref=f3e221]: 2b5cd59e-1966-4ab3-bc93-b01bf2f6ae81
                - listitem [ref=f3e222]: 4e851a6d-705a-4d1e-a2d8-ea4095451898
                - listitem [ref=f3e223]: 5f19affd-5080-41b4-a3f7-9a2afc6c0f2d
                - listitem [ref=f3e224]: a92df4ad-b750-45d7-8781-3727008c471a
                - listitem [ref=f3e225]: 971a7a61-a6df-49cc-b96b-fcf8249f8796
                - listitem [ref=f3e226]: 19aa056d-bcb7-4ff2-910d-d3d0cd401c27
                - listitem [ref=f3e227]: 373041bb-d2ba-4f28-bb8c-a9cc644fa65a
                - listitem [ref=f3e228]: b6932109-ef87-4e64-8779-ebc6d6c248a5
                - listitem [ref=f3e229]: 8c1a5bd6-ffc1-47b7-a7bb-715928c194b0
                - listitem [ref=f3e230]: c3018881-dc52-482a-a237-2d2465b1155f
                - listitem [ref=f3e231]: c10e86ec-0c4f-40b2-8fdf-380d24a07f9e
                - listitem [ref=f3e232]: 45beab43-6f44-498f-8bb8-15bd70b3caf4
                - listitem [ref=f3e233]: 7fabf211-5b4d-442a-a6e8-0c211005ce60
                - listitem [ref=f3e234]: b3fde7e2-032c-4a69-9953-8687c02d2d64
                - listitem [ref=f3e235]: 91488bd2-b52b-4b70-8548-5c9e71ab57bd
                - listitem [ref=f3e236]: 5eebdca8-aea0-4867-af88-d81b7258deef
                - listitem [ref=f3e237]: aed04172-eb15-4120-9699-9b01a5fd6738
                - listitem [ref=f3e238]: 20b13428-6c2d-4b82-813f-11881ed41c8b
                - listitem [ref=f3e239]: c87a0a5b-8fbd-4e7b-9c93-57ba35536372
                - listitem [ref=f3e240]: 5c8580ee-a215-4bd7-9d42-f1c655f188ce
                - listitem [ref=f3e241]: ccafa4c1-17b3-42cd-bba8-a76ca07a59c9
                - listitem [ref=f3e242]: f752cf5a-55e6-4edd-84f7-600157fe3f0d
                - listitem [ref=f3e243]: 91d26bbb-f4c5-480e-be4b-462a248cb00f
                - listitem [ref=f3e244]: d2820701-cc9b-4ddc-98dc-c3603b877d27
                - listitem [ref=f3e245]: 131c06ec-d986-4a02-a580-dabae2db655e
                - listitem [ref=f3e246]: 40ef9308-c616-44e4-ab05-967a23771278
                - listitem [ref=f3e247]: 7a0d02c1-fcd5-43cd-bd13-2d9913d3972d
                - listitem [ref=f3e248]: da706737-eda3-4d62-be5e-bd6c4927fb25
                - listitem [ref=f3e249]: 576099a2-a258-496a-ae90-60f912635241
                - listitem [ref=f3e250]: 58b27da6-e1ab-4330-848b-4e857b9cde86
                - listitem [ref=f3e251]: ee6c7079-c97c-40a4-a076-fb7e84a93c4f
                - listitem [ref=f3e252]: ac785865-a41e-4d75-884c-e93bd3d170ed
                - listitem [ref=f3e253]: 9abe03b1-e090-4914-bdc9-d38a535572e4
                - listitem [ref=f3e254]: 76808704-5c5a-4520-a7da-97130384526b
                - listitem [ref=f3e255]: 9228dee2-a67f-4baf-b36f-0c3e316c89ac
                - listitem [ref=f3e256]: 706f4196-c96d-4897-918b-d8aeec8a16d4
                - listitem [ref=f3e257]: 982b4791-0d80-432c-ab9a-bf2c37fc4fbd
                - listitem [ref=f3e258]: 198bea6f-4a63-48d0-a1fd-8e61e77c0a1c
                - listitem [ref=f3e259]: 2d97cb7d-f935-45b3-b4b5-6709e3201897
                - listitem [ref=f3e260]: 5ea8bbce-28cf-4a5c-8965-09dba7c119de
                - listitem [ref=f3e261]: e69e66fa-8488-4c3c-9b80-1afd7ee10f79
                - listitem [ref=f3e262]: 59e76b44-3276-4b7c-9070-f4dee7ffecd1
                - listitem [ref=f3e263]: 6ebf3f24-260c-40bf-b56a-503a5c2aa897
                - listitem [ref=f3e264]: 58a20ef2-565b-4d11-b029-aaa254cebbcf
                - listitem [ref=f3e265]: d1966f4e-7181-4935-9265-4f941ec56944
                - listitem [ref=f3e266]: f70581c7-2cb4-44aa-9088-d3bc167f55f6
                - listitem [ref=f3e267]: 43736b8c-12ca-4c53-b6f4-117add375aed
                - listitem [ref=f3e268]: f9e26c2b-7ae5-4c16-be02-c2797b3f0adb
                - listitem [ref=f3e269]: 93be64b4-7cf4-479e-81c6-9e105413acf0
                - listitem [ref=f3e270]: 6b652ef9-9d51-448b-b93f-f4ab1d02f243
                - listitem [ref=f3e271]: e4a2a0df-8d91-4e5b-b07b-d309872adac7
                - listitem [ref=f3e272]: 29624aa0-8f13-4ab2-b5f3-e296e0785b00
                - listitem [ref=f3e273]: 83485b9a-fa21-4bfb-99ca-08f8baef65b9
                - listitem [ref=f3e274]: c4b7987b-799a-475c-9f2e-7e7f1e428d41
                - listitem [ref=f3e275]: c3a2d67c-aa41-44eb-bfb7-526bdba011cb
                - listitem [ref=f3e276]: 0637af6f-0011-49e8-bb70-13ba9d64d8dd
                - listitem [ref=f3e277]: 797a152c-ceed-49fb-a819-5b7a8f28846c
                - listitem [ref=f3e278]: 4162f4bd-b875-415c-a2a5-c2c11a3d7b2a
                - listitem [ref=f3e279]: c39b28ec-663e-421d-806f-e8a36bc38452
                - listitem [ref=f3e280]: 3bb23b2e-38c6-448f-9343-08eb8e1649c0
                - listitem [ref=f3e281]: fe902d00-8b82-4ff2-af4a-d8222c12eab2
                - listitem [ref=f3e282]: b5f0cae6-55b0-48da-bfcf-0bf2b4a6f745
            - generic [ref=f3e283]:
              - generic [ref=f3e284]:
                - term [ref=f3e285]: 所選期間內推廣此直播貼文的廣告花費
                - definition [ref=f3e286]: NT$17.30
              - generic [ref=f3e287]:
                - term [ref=f3e288]: 已收款訂單
                - definition [ref=f3e289]: "4"
              - generic [ref=f3e290]:
                - term [ref=f3e291]: 扣除退款後淨營收
                - definition [ref=f3e292]: NT$149
              - generic [ref=f3e293]:
                - term [ref=f3e294]: 待收款訂單
                - definition [ref=f3e295]: "1"
              - generic [ref=f3e296]:
                - term [ref=f3e297]: 待收款金額
                - definition [ref=f3e298]: NT$75
              - generic [ref=f3e299]:
                - term [ref=f3e300]: 訂單 ROAS
                - definition [ref=f3e301]: 8.61×
            - paragraph [ref=f3e302]: "多個推廣同時進行，未分配到單一廣告: 1"
            - heading "留言 → 喊單 → 結帳連結 → 已收款訂單" [level=3] [ref=f3e303]
            - region "留言 · 喊單 · 結帳連結 · 已付款／收款訂單" [ref=f3e304]:
              - table [ref=f3e305]:
                - rowgroup [ref=f3e306]:
                  - row [ref=f3e307]:
                    - columnheader "留言" [ref=f3e308]
                    - columnheader "喊單" [ref=f3e309]
                    - columnheader "結帳連結" [ref=f3e310]
                    - columnheader "已付款／收款訂單" [ref=f3e311]
                - rowgroup [ref=f3e312]:
                  - row [ref=f3e313]:
                    - cell "9" [ref=f3e314]
                    - cell "9" [ref=f3e315]
                    - cell "9" [ref=f3e316]
                    - cell "4" [ref=f3e317]
            - heading "同一場次時間軸" [level=3] [ref=f3e318]
            - paragraph [ref=f3e319]: 每小時廣告花費以絕對時間對齊；Meta 觀眾統計不與個別買家連結。
            - region "時間（Asia/Taipei） · 廣告花費 · 觀眾 · 留言 · 喊單 · 已收款訂單 · 扣除退款後淨營收" [ref=f3e320]:
              - table [ref=f3e321]:
                - rowgroup [ref=f3e322]:
                  - row [ref=f3e323]:
                    - columnheader "時間（Asia/Taipei）" [ref=f3e324]
                    - columnheader "廣告花費" [ref=f3e325]
                    - columnheader "觀眾" [ref=f3e326]
                    - columnheader "留言" [ref=f3e327]
                    - columnheader "喊單" [ref=f3e328]
                    - columnheader "已收款訂單" [ref=f3e329]
                    - columnheader "扣除退款後淨營收" [ref=f3e330]
                - rowgroup [ref=f3e331]:
                  - row [ref=f3e332]:
                    - rowheader "2026/10/04 14:57" [ref=f3e333]
                    - cell "未提供" [ref=f3e334]
                    - cell "未提供" [ref=f3e335]
                    - cell "9" [ref=f3e336]
                    - cell "9" [ref=f3e337]
                    - cell "4" [ref=f3e338]
                    - cell "NT$149" [ref=f3e339]
                  - row [ref=f3e340]:
                    - rowheader "2026/10/05 04:00" [ref=f3e341]
                    - cell "NT$17.30" [ref=f3e342]
                    - cell "未提供" [ref=f3e343]
                    - cell "0" [ref=f3e344]
                    - cell "0" [ref=f3e345]
                    - cell "0" [ref=f3e346]
                    - cell "NT$0" [ref=f3e347]
            - generic [ref=f3e348]:
              - generic [ref=f3e349]:
                - heading "觀眾輪廓（Meta 統計）" [level=3] [ref=f3e350]
                - button "刷新觀眾洞察" [ref=f3e352] [cursor=pointer]
                - generic [ref=f3e353]:
                  - generic [ref=f3e354]:
                    - term [ref=f3e355]: 影片觀看次數
                    - definition [ref=f3e356]: "34"
                  - generic [ref=f3e357]:
                    - term [ref=f3e358]: 最高同時觀看人數
                    - definition [ref=f3e359]: 未提供
                  - generic [ref=f3e360]:
                    - term [ref=f3e361]: 總觀看時間（毫秒）
                    - definition [ref=f3e362]: 未提供
                - region "年齡與性別 · 觀看時間（毫秒），非人數" [ref=f3e363]:
                  - table [ref=f3e364]:
                    - rowgroup [ref=f3e365]:
                      - row [ref=f3e366]:
                        - columnheader "年齡與性別" [ref=f3e367]
                        - columnheader "觀看時間（毫秒），非人數" [ref=f3e368]
                    - rowgroup [ref=f3e369]:
                      - row [ref=f3e370]:
                        - rowheader "F.25-34" [ref=f3e371]
                        - cell "1234" [ref=f3e372]
                - region "地區 · 觀看時間（毫秒），非人數" [ref=f3e373]:
                  - table [ref=f3e374]:
                    - rowgroup [ref=f3e375]:
                      - row [ref=f3e376]:
                        - columnheader "地區" [ref=f3e377]
                        - columnheader "觀看時間（毫秒），非人數" [ref=f3e378]
                    - rowgroup [ref=f3e379]:
                      - row [ref=f3e380]:
                        - rowheader "Taipei" [ref=f3e381]
                        - cell "4321" [ref=f3e382]
              - generic [ref=f3e383]:
                - heading "買家分佈（訂單實績）" [level=3] [ref=f3e384]
                - generic [ref=f3e385]:
                  - generic [ref=f3e386]:
                    - term [ref=f3e387]: 新買家
                    - definition [ref=f3e388]: "3"
                  - generic [ref=f3e389]:
                    - term [ref=f3e390]: 回購買家
                    - definition [ref=f3e391]: "1"
                  - generic [ref=f3e392]:
                    - term [ref=f3e393]: 平均訂單金額
                    - definition [ref=f3e394]: NT$37.25
                - region "收件縣市 · 已收款訂單 · 扣除退款後淨營收" [ref=f3e395]:
                  - table [ref=f3e396]:
                    - rowgroup [ref=f3e397]:
                      - row [ref=f3e398]:
                        - columnheader "收件縣市" [ref=f3e399]
                        - columnheader "已收款訂單" [ref=f3e400]
                        - columnheader "扣除退款後淨營收" [ref=f3e401]
                    - rowgroup [ref=f3e402]:
                      - row [ref=f3e403]:
                        - rowheader "臺北市" [ref=f3e404]
                        - cell "3" [ref=f3e405]
                        - cell "NT$124" [ref=f3e406]
                      - row [ref=f3e407]:
                        - rowheader "—" [ref=f3e408]
                        - cell "1" [ref=f3e409]
                        - cell "NT$25" [ref=f3e410]
                - heading "熱銷商品" [level=4] [ref=f3e411]
                - region "商品 · 數量" [ref=f3e412]:
                  - table [ref=f3e413]:
                    - rowgroup [ref=f3e414]:
                      - row [ref=f3e415]:
                        - columnheader "商品" [ref=f3e416]
                        - columnheader "數量" [ref=f3e417]
                    - rowgroup [ref=f3e418]:
                      - row [ref=f3e419]:
                        - rowheader "t04-ce8a746d435d" [ref=f3e420]
                        - cell "8" [ref=f3e421]
                - heading "每分鐘訂單" [level=4] [ref=f3e422]
                - region "時間（Asia/Taipei） · 已收款訂單 · 扣除退款後淨營收" [ref=f3e423]:
                  - table [ref=f3e424]:
                    - rowgroup [ref=f3e425]:
                      - row [ref=f3e426]:
                        - columnheader "時間（Asia/Taipei）" [ref=f3e427]
                        - columnheader "已收款訂單" [ref=f3e428]
                        - columnheader "扣除退款後淨營收" [ref=f3e429]
                    - rowgroup [ref=f3e430]:
                      - row [ref=f3e431]:
                        - rowheader "2026/10/04 14:57" [ref=f3e432]
                        - cell "4" [ref=f3e433]
                        - cell "NT$149" [ref=f3e434]
        - paragraph [ref=f3e436]: DaWan Live is operated by Hong Kong Da Wan Trading Limited
  - alert [ref=f3e437]
```

# Test source

```ts
  619 |   const file = `attribution-${locale}-${width}${state ? `-${state}` : ""}.png`;
  620 |   await page.screenshot({ path: path.join(evidence, file), fullPage: true });
  621 |   const manifestPath = path.join(evidence, "screenshots.json");
  622 |   let manifest: {
  623 |     File: string;
  624 |     Sha256: string;
  625 |     Locale: string;
  626 |     Viewport: string;
  627 |   }[] = [];
  628 |   try {
  629 |     manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  630 |   } catch (error) {
  631 |     if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  632 |   }
  633 |   manifest.push({
  634 |     File: file,
  635 |     Sha256: createHash("sha256")
  636 |       .update(await readFile(path.join(evidence, file)))
  637 |       .digest("hex"),
  638 |     Locale: locale,
  639 |     Viewport: String(width),
  640 |   });
  641 |   await writeFile(manifestPath, JSON.stringify(manifest, null, 2));
  642 | }
  643 | 
  644 | for (const locale of ["en", "zh-TW", "zh-CN"] as const)
  645 |   for (const width of [390, 1586] as const) {
  646 |     test(`AT7/AT9 actual report clicks ${locale} ${width}`, async ({
  647 |       page,
  648 |     }) => {
  649 |       const c = attributionCopy[locale],
  650 |         ledger: {
  651 |           control: string;
  652 |           action: string;
  653 |           expected: string;
  654 |           actual: string;
  655 |         }[] = [];
  656 |       try {
  657 |         await page.setViewportSize({ width, height: 992 });
  658 |         await login(page, width);
  659 |         ledger.push({
  660 |           control: "identity sign-in / mobile navigation",
  661 |           action: width === 390 ? "click/open/assert/close" : "click/assert",
  662 |           expected:
  663 |             "authenticated marketing navigation visible; mobile drawer closed",
  664 |           actual: "PASS",
  665 |         });
  666 |         await page.goto(`/${locale}/ads?store=${store}`);
  667 |         await page.getByTestId("ads-attribution-link").click();
  668 |         await expect(page.getByTestId("ads-attribution")).toBeVisible();
  669 |         ledger.push({
  670 |           control: "ads-attribution-link",
  671 |           action: "click",
  672 |           expected: "report page",
  673 |           actual: "PASS",
  674 |         });
  675 |         await page.getByTestId("attribution-from").fill(fixture.from);
  676 |         await page.getByTestId("attribution-to").fill(fixture.to);
  677 |         const reportRead = page.waitForResponse(
  678 |           (r) =>
  679 |             r.request().method() === "GET" &&
  680 |             new URL(r.url()).pathname ===
  681 |               `/api/stores/${store}/ads/attribution`,
  682 |         );
  683 |         await page.getByTestId("attribution-apply").click();
  684 |         const reportData = await (await reportRead).json();
  685 |         const linkedIDs: string[] = reportData.sessions.find(
  686 |           (s: { session_id: string }) => s.session_id === fixture.session_id,
  687 |         ).draft_ids;
  688 |         expect(linkedIDs.length).toBeGreaterThanOrEqual(100);
  689 |         await expect(page.getByTestId("attribution-window")).toContainText(
  690 |           `${fixture.from} – ${fixture.to}`,
  691 |         );
  692 |         ledger.push({
  693 |           control: "from/to/apply",
  694 |           action: "fill/fill/click",
  695 |           expected: "fixture date window",
  696 |           actual: "PASS",
  697 |         });
  698 |         await page
  699 |           .getByTestId("attribution-draft")
  700 |           .selectOption(fixture.draft_id);
  701 |         await expect(page).toHaveURL(new RegExp(`draft=${fixture.draft_id}`));
  702 |         await page
  703 |           .getByTestId("attribution-session")
  704 |           .selectOption(fixture.session_id);
  705 |         await expect(page).toHaveURL(
  706 |           new RegExp(`session=${fixture.session_id}`),
  707 |         );
  708 |         const linked = page.getByTestId("attribution-linked-drafts");
  709 |         const disclosure = linked.locator("summary");
  710 |         await expect(linked).not.toHaveAttribute("open", "");
  711 |         await expect(disclosure).toHaveText(
  712 |           `${c.draftIds} (${linkedIDs.length})`,
  713 |         );
  714 |         expect((await disclosure.boundingBox())!.height).toBeGreaterThanOrEqual(
  715 |           44,
  716 |         );
  717 |         await disclosure.click();
  718 |         await expect(linked).toHaveAttribute("open", "");
> 719 |         await expect(linked.locator("li")).toHaveText(linkedIDs);
      |                                            ^ Error: expect(locator).toHaveText(expected) failed
  720 |         await disclosure.focus();
  721 |         await page.keyboard.press("Enter");
  722 |         await expect(linked).not.toHaveAttribute("open", "");
  723 |         ledger.push({
  724 |           control: "linked-draft identifiers",
  725 |           action: "click/compare server ids/keyboard close",
  726 |           expected:
  727 |             "44px disclosure; full server list retained; collapsed report stays compact",
  728 |           actual: "PASS",
  729 |         });
  730 |         await visibleFacts(page, locale);
  731 |         ledger.push({
  732 |           control: "audience/hourly/privacy facts",
  733 |           action: "read DOM",
  734 |           expected: "exact runner values, nulls unknown, no private strings",
  735 |           actual: "PASS",
  736 |         });
  737 |         ledger.push({
  738 |           control: "draft/session",
  739 |           action: "selectOption/selectOption",
  740 |           expected: "exact fixture facts",
  741 |           actual: "PASS",
  742 |         });
  743 |         await audienceStateClicks(page, locale, width);
  744 |         await unknownDraftClicks(page, locale, width);
  745 |         ledger.push({
  746 |           control:
  747 |             "unknown draft, capped report, unavailable breakdowns, Facebook reconnect entry",
  748 |           action: "selectOption/reload/selectOption/click/reload",
  749 |           expected:
  750 |             "spend and ROAS remain — after reload; omitted metrics stay unknown; cap and unavailable labels remain visible; settings Page-connect entry opens",
  751 |           actual: "PASS",
  752 |         });
  753 |         ledger.push({
  754 |           control:
  755 |             "insufficient/not_authorized sessions + Los Angeles/provisional/promoted-post labels + ROAS",
  756 |           action: "selectOption/reload/selectOption/reload/selectOption",
  757 |           expected:
  758 |             "both exact localized states survive reload; unknown metrics stay unknown; primary report facts restored",
  759 |           actual: "PASS",
  760 |         });
  761 |         const observeRead = (session: string = fixture.session_id) =>
  762 |           page.waitForResponse(
  763 |             (r) =>
  764 |               r.request().method() === "POST" &&
  765 |               new URL(r.url()).pathname ===
  766 |                 `/api/stores/${store}/ads/sessions/${session}/audience-read`,
  767 |           );
  768 |         // R12: real unboosted claim + consented card Begin/capture remains in
  769 |         // its session cohort, without crediting any ad/draft.
  770 |         await page
  771 |           .getByTestId("attribution-session")
  772 |           .selectOption(fixture.organic.session_id);
  773 |         const organicPanel = page.getByTestId("attribution-session-panel");
  774 |         const organicFacts = organicPanel.getByTestId(
  775 |           "attribution-session-facts",
  776 |         );
  777 |         for (const [label, value] of [
  778 |           [c.orders, "1"],
  779 |           [c.net, money(locale, "TWD", fixture.organic.net_minor)],
  780 |           [c.pending, "0"],
  781 |         ]) {
  782 |           await expect(
  783 |             organicFacts
  784 |               .locator("div")
  785 |               .filter({ has: page.getByText(label, { exact: true }) })
  786 |               .locator("dd"),
  787 |           ).toHaveText(value);
  788 |         }
  789 |         await expect(
  790 |           organicPanel.getByTestId("attribution-linked-drafts"),
  791 |         ).toHaveCount(0);
  792 |         await reportShot(page, locale, width, "organic-cohort");
  793 |         ledger.push({
  794 |           control: "organic claim cohort",
  795 |           action: "selectOption/read",
  796 |           expected:
  797 |             "one real paid unboosted claim order, exact net, no linked drafts",
  798 |           actual: "PASS",
  799 |         });
  800 |         // Acknowledged UNKNOWN is a read-only receipt. Reload retains it but
  801 |         // does not disable a new intention; the server replays during cooldown.
  802 |         await page
  803 |           .getByTestId("attribution-session")
  804 |           .selectOption(fixture.state_sessions.not_read);
  805 |         await expect(page.getByTestId("attribution-not-read")).toHaveText(
  806 |           c.notRead,
  807 |         );
  808 |         await expect(page.getByTestId("attribution-reconnect")).toHaveCount(0);
  809 |         let unknownKey = "";
  810 |         for (let i = 0; i < 2; i++) {
  811 |           const unknownRead = observeRead(fixture.state_sessions.not_read);
  812 |           await page.getByTestId("attribution-audience-refresh").click();
  813 |           const response = await unknownRead;
  814 |           expect(response.ok()).toBe(true);
  815 |           expect(await response.json()).toEqual({
  816 |             operation_id: fixture.unknown_audience_operation_id,
  817 |             state: "UNKNOWN",
  818 |           });
  819 |           const key = response.request().headers()["idempotency-key"];
```