# Project Overview

This project is a Go backend server designed to power a multi-platform Flutter application. It handles business logic for a system involving admins, merchants, staff, customers, and inventory management.

## Business Logic Rules

The following rules define the core relationships and permissions within the system.

### User Roles & Permissions

1.  **Admin**:
    *   Can create other Admins, Merchants, and Staff.
    *   Can assign Staff to any Merchant.

2.  **Merchant**:
    *   Can create multiple Shops.
    *   Can create Suppliers (optional, for record-keeping during stock-in).
    *   Can assign their own Staff to one of their own Shops.

3.  **Staff**:
    *   Can be assigned to only one Merchant and one Shop.

4.  **Supplier**:
    *   Created by a Merchant, primarily for recording stock-in. Not a mandatory entity.

5.  **Customer**:
    *   Created automatically during a purchase transaction at a Shop.

### Entity Relationships

*   **Shops & Inventories**: Each Shop must have its own dedicated Inventory, created automatically when the Shop is created.
