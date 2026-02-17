# DocShare Roadmap

This document outlines the planned features and improvements for DocShare.

## Goals

- Keep it simple to set up and use
- Stay lean but feature-complete
- Prioritize security without sacrificing usability

---

## Short Term

Near-term priorities for the next few releases:

1. ~~**Background preview generation** — Move document preview generation to a job queue to improve upload response times~~ ✅ Completed
2. ~~**Pagination** — Add to all list endpoints for better performance with large datasets~~ ✅ Completed
3. **Rate limiting** — Prevent API abuse

---

## Medium Term

Features planned for the medium term:

1. ~~**OAuth/SSO** — Support for single sign-on with providers like Google, GitHub, Microsoft Entra ID~~ ✅ Completed
2. **Virus scanning** — Integrate ClamAV or similar for file content scanning
3. **File versioning** — Keep history of file changes
4. **Trash/recovery** — Soft delete with restore capability

---

## Long Term

 aspirational goals for future versions:

1. **Real-time collaboration** — WebRTC or WebSocket for live editing
2. **Mobile apps** — React Native or native iOS/Android
3. **Advanced permissions** — Custom permission combinations
4. **Multi-tenant** — Support multiple organizations
5. ~~**Multi-factor authentication** — Enhanced account security~~ 5. ✅ Completed

---

*Have a feature request? Open an issue on GitHub.*
