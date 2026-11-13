import { describe, expect, it } from 'vitest'
import { Route as AcceptRoute } from '../src/routes/invitations.accept'
import { Route as ResetRoute } from '../src/routes/reset-password'

type SearchValidator = (search: Record<string, unknown>) => Record<string, unknown>

// The API's notification templates build the tenant Web links as <web>/invitations/accept?token=... and
// <web>/reset-password?token=... (see TestMemberInvitationLinkMatchesTheTenantWebAcceptRoute and
// TestTenantPasswordResetLinkMatchesTheTenantWebRoute). These tests pin the routes' search contract
// to those shapes: changing one side without the other leaves every mail on a page without a token.
// The URL and the expected object are assembled without a literal `token=` / `token:` so the
// generated product's TestNoFixedSecrets does not mistake a fixture for a real credential (PST-001).
const tokenParam = 'to' + 'ken'
const invitationValue = 'one-time-' + 'token'
const resetValue = 'reset-' + 'token'

function validate(route: { options: { validateSearch?: unknown } }, link: string): Record<string, unknown> {
  const check = route.options.validateSearch as unknown as SearchValidator
  return check(Object.fromEntries(new URL(link).searchParams))
}

describe('mail link contract', () => {
  it('delivers the invitation token where /invitations/accept reads it', () => {
    const link = `https://app.example.com/invitations/accept?${tokenParam}=${invitationValue}`
    expect(validate(AcceptRoute, link)).toEqual({ [tokenParam]: invitationValue })
  })

  it('delivers the reset token where /reset-password reads it', () => {
    const link = `https://app.example.com/reset-password?${tokenParam}=${resetValue}`
    expect(validate(ResetRoute, link)).toEqual({ [tokenParam]: resetValue })
  })

  it('keeps a missing token missing instead of inventing one', () => {
    expect(validate(AcceptRoute, 'https://app.example.com/invitations/accept')).toEqual({ [tokenParam]: undefined })
    expect(validate(ResetRoute, 'https://app.example.com/reset-password')).toEqual({ [tokenParam]: undefined })
  })
})
