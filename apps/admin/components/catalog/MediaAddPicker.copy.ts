// Purpose: typed three-locale copy of the signed 媒体上传 picker (6:22038 / 6:22147); zh-TW is the signed text verbatim.
// Depends on: Locale from @live-commerce/i18n.
// Used by: MediaAddPicker and the CatalogSimplifiedEditor entry button.
import type { Locale } from "@live-commerce/i18n";
export const mediaAddPickerCopy = {
  "zh-TW": {
    open: "加入圖片",
    title: "選擇要加入的圖片",
    hint: "JPG、PNG 或 WebP；每張不超過 5 MB。加入後仍需儲存商品。",
    select: "選擇此圖片",
    cancel: "取消",
    apply: "加入所選圖片",
  },
  "zh-CN": {
    open: "添加图片",
    title: "选择要添加的图片",
    hint: "JPG、PNG 或 WebP；每张不超过 5 MB。添加后仍需保存商品。",
    select: "选择此图片",
    cancel: "取消",
    apply: "添加所选图片",
  },
  en: {
    open: "Add images",
    title: "Choose images to add",
    hint: "JPG, PNG or WebP; up to 5 MB each. Save the product after adding.",
    select: "Select this image",
    cancel: "Cancel",
    apply: "Add selected images",
  },
} satisfies Record<Locale, { open: string; title: string; hint: string; select: string; cancel: string; apply: string }>;
