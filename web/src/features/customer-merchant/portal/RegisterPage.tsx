import { useState } from 'react';
import { Link } from 'react-router-dom';
import { errorMessage, portalRequest } from './api';
import { PortalErrorMessage, PortalLayout } from './PortalLayout';

const fields = [
  ['full_name', 'Full name', 'text', 'name'],
  ['email', 'Email', 'email', 'email'],
  ['phone', 'Mobile number', 'tel', 'tel'],
  ['password', 'Password', 'password', 'new-password'],
] as const;

export function RegisterPage() {
  const [complete, setComplete] = useState(false);
  const [error, setError] = useState('');
  const [pending, setPending] = useState(false);
  return (
    <PortalLayout title="Create a customer account" profile="public">
      {complete ? <section className="m6-card" role="status">
        <h2>Account created</h2><Link to="/auth/login">Sign in to SwiftDrop</Link>
      </section> : <form className="m6-card" onSubmit={async event => {
        event.preventDefault();
        if (pending) return;
        const body = Object.fromEntries(new FormData(event.currentTarget));
        setPending(true);
        setError('');
        try {
          await portalRequest('/auth/register', { method: 'POST', body });
          setComplete(true);
        } catch (cause) {
          setError(errorMessage(cause));
        } finally { setPending(false); }
      }}>
        <fieldset className="m6-grid" disabled={pending}>
          {fields.map(([name, label, type, autoComplete]) => <label key={name} className="m6-field">{label}
            <input name={name} type={type} required autoComplete={autoComplete} data-testid={`register-${name}-input`} />
          </label>)}
        </fieldset>
        <PortalErrorMessage message={error} />
        <div className="m6-actions"><button className="m6-btn" disabled={pending} data-testid="register-submit-btn">
          {pending ? 'Creating account…' : 'Create account'}
        </button><Link to="/auth/login">Already have an account?</Link></div>
      </form>}
    </PortalLayout>
  );
}
