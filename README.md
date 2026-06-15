# Project Overview

This project is a Go backend server designed to power a multi-platform Flutter application. It handles business logic for a system involving admins, merchants, staff, customers, and inventory management.

## Business Logic Rules

### User Roles & Permissions

1.  **Admin**: Can create other Admins, Merchants, and Staff. Can assign Staff to any Merchant.
2.  **Merchant**: Can create multiple Shops. Can create Suppliers. Can assign their own Staff to one of their own Shops.
3.  **Staff**: Can be assigned to only one Merchant and one Shop.
4.  **Supplier**: Created by a Merchant for recording stock-in.
5.  **Customer**: Created automatically during a purchase transaction.

### Entity Relationships

*   **Shops & Inventories**: Each Shop must have its own dedicated Inventory.

---

