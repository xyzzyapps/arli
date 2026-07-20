"""Environment for arli â€” maps symbols to values."""

from __future__ import annotations
from typing import Any, Optional

from .types import Symbol


class Environment:
    """A nested scope environment for arli.

    Supports:
    - Variable definition and lookup
    - Lexical scoping via parent environments
    - Mutation via set!
    """

    def __init__(self, parent: Optional[Environment] = None,
                 name: str = "global") -> None:
        self.parent = parent
        self.name = name
        self._bindings: dict[str, Any] = {}

    def define(self, name: str, value: Any) -> Any:
        """Define a new binding in this environment."""
        self._bindings[name] = value
        return value

    def lookup(self, name: str) -> Optional[Any]:
        """Look up a symbol in this environment or its parents."""
        if name in self._bindings:
            return self._bindings[name]
        if self.parent is not None:
            return self.parent.lookup(name)
        return None

    def get(self, name: str) -> Any:
        """Look up a symbol; raise NameError if not found."""
        val = self.lookup(name)
        if val is None:
            raise NameError(f"Undefined symbol: {name}")
        return val

    def set(self, name: str, value: Any) -> Any:
        """Mutate an existing binding. Walks up to find it."""
        if name in self._bindings:
            self._bindings[name] = value
            return value
        if self.parent is not None:
            return self.parent.set(name, value)
        raise NameError(f"Cannot set! undefined symbol: {name}")

    def has(self, name: str) -> bool:
        """Check if a symbol is bound."""
        if name in self._bindings:
            return True
        if self.parent is not None:
            return self.parent.has(name)
        return False

    def extend(self, name: str = "block") -> Environment:
        """Create a child environment."""
        return Environment(parent=self, name=name)

    def __repr__(self) -> str:
        return f"<Env {self.name}: {dict(self._bindings)}>"
