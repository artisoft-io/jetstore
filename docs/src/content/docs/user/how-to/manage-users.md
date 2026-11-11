---
title: Manage users
description: Activate a newly registered account and give it roles, change an account's roles, or delete an account, on the User Administration screen.
kind: how-to
sidebar:
  order: 10
---

**Before you start**: your account has `user_admin`. [Users, roles and capabilities](/docs/user/concepts/users-and-capabilities/)
explains which role grants what.

## Activate a new account

1. **The person registers.** On the sign-in page they choose **Create one**, and enter their name,
   email and a password of at least 14 characters. The account is created inactive.

   ![The Create an account form, with the name Robin Ashby and the email robin.ashby@localhost.test filled in and the password fields empty.](../../../../assets/screenshots/user/how-to/manage-users/register.png)

2. Open the brand menu, **JetStore Workspace**, and choose **User Administration**. The new account is
   listed with **Active User?** at `0`.

   ![User Administration: the registered accounts, with Robin Ashby ticked and not active.](../../../../assets/screenshots/user/how-to/manage-users/registered-users.png)

3. Tick the account and press **Update User Profile**.
4. Set **Active User?** to **Active**, and tick the roles it should hold: here `ops_user`, to load
   files and run pipelines. Press **Submit**.

   ![The Update User Profile dialog for Robin Ashby: Active selected, and the role ops_user ticked among client_advocate, designated_admin, knowledge_engineer, ops_user and system_role.](../../../../assets/screenshots/user/how-to/manage-users/update-user-profile.png)

**How to tell it worked**: the account shows **Active User?** at `1`, and the person can now sign in.

![User Administration with Robin Ashby now active.](../../../../assets/screenshots/user/how-to/manage-users/user-active.png)

## Change an account's roles, or deactivate it

Tick the account, press **Update User Profile**, change the roles or set **Active User?** to
**Inactive**, and press **Submit**.

## Delete an account

Tick the account, press **Delete User**, and confirm with **OK**. The account leaves the list.

The installation's administrator account cannot be changed or deleted here, so the installation
always keeps an account that can manage users. The [User Administration](/docs/user/screens/user-administration/)
page describes the screen, and [Sign in](/docs/user/screens/sign-in/) the registration form.
