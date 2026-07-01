# Coupon eligibility

The coupon service should prevent ineligible users from claiming a coupon. The first v0.1 dogfood demand is intentionally small enough for the backend-demand loop to process in one sitting.

## 目标

- Active members can claim coupons when the coupon is available.
- Inactive members are blocked with a clear business reason.

## 业务规则

- User status must be active before a coupon claim succeeds.
- The coupon must be inside the valid claim window.

## 验收标准

- Given an active member and an available coupon, the claim succeeds.
- Given an inactive member, the claim fails and records the eligibility reason.

## 待确认

- Confirm the exact business error code for inactive members.