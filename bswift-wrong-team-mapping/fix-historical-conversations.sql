-- Fix historical conversations for Briana Joyner and Tennisha Cox
-- Target: PostgreSQL bswift-us-east-1 (app.chats)
-- Connection: cresta-cli connstring -i us-east-1-prod us-east-1-prod bswift-us-east-1
--
-- These agents had duplicate GUEST_USER accounts that received all conversations.
-- New conversations (post ~Mar 18 2026) already go to PROD users.
-- This script migrates historical conversations from GUEST → PROD user IDs
-- and sets the correct team_group_id.

BEGIN;

-- ============================================================
-- Briana Joyner
-- GUEST user_id: 516b107b2e23b60f  (1,752 conversations, Sep 2025 – Mar 17 2026)
-- PROD  user_id: e41f4a45cbde99a6
-- Correct team:  01992ac1-2405-77f7-bae6-84114a04211e (Nathaniel King)
-- ============================================================

-- Dry-run: verify row count before update
-- SELECT count(*) FROM app.chats WHERE agent_user_id = '516b107b2e23b60f';
-- Expected: 1752

UPDATE app.chats
SET agent_user_id = 'e41f4a45cbde99a6',
    team_group_id = '01992ac1-2405-77f7-bae6-84114a04211e'
WHERE agent_user_id = '516b107b2e23b60f';

-- ============================================================
-- Tennisha Cox
-- GUEST user_id: f22a48d63d182d43  (418 conversations, Mar 3 – Mar 17 2026)
-- PROD  user_id: 56aa04f69c744cb9
-- Correct team:  01992ac1-249c-74cc-8b24-b4f9d077506f (Tavian Mozie)
-- ============================================================

-- Dry-run: verify row count before update
-- SELECT count(*) FROM app.chats WHERE agent_user_id = 'f22a48d63d182d43';
-- Expected: 418

UPDATE app.chats
SET agent_user_id = '56aa04f69c744cb9',
    team_group_id = '01992ac1-249c-74cc-8b24-b4f9d077506f'
WHERE agent_user_id = 'f22a48d63d182d43';

COMMIT;

-- ============================================================
-- Post-fix verification
-- ============================================================

-- Should return 0 rows for both GUEST user IDs:
-- SELECT count(*) FROM app.chats WHERE agent_user_id = '516b107b2e23b60f';
-- SELECT count(*) FROM app.chats WHERE agent_user_id = 'f22a48d63d182d43';

-- Should show all conversations under PROD user IDs with correct team:
-- SELECT agent_user_id, team_group_id, count(*) as convo_count
-- FROM app.chats
-- WHERE agent_user_id IN ('e41f4a45cbde99a6', '56aa04f69c744cb9')
-- GROUP BY agent_user_id, team_group_id
-- ORDER BY agent_user_id;
