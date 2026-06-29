# Demand: Coupon Eligibility Check

## Business Context

The coupon service should prevent ineligible users from claiming a coupon. The first v0.1 dogfood demand is intentionally small enough for the backend-demand loop to process in one sitting.

## Requirement Input

Only active members can claim coupons. A user is eligible when all of these conditions are true:

- The user exists.
- The user status is `active`.
- The coupon exists.
- The coupon is not expired.
- The user has not claimed the same coupon before.

The API should return a clear rejection reason when any condition fails.

## Acceptance Criteria

- Eligible active members can claim an unexpired coupon once.
- Inactive users are rejected.
- Missing users are rejected.
- Missing coupons are rejected.
- Expired coupons are rejected.
- Duplicate claims are rejected.
- Tests cover each rejection reason.
